// Package kafka implements the event publisher and an at-least-once consumer
// with bounded retries and a dead-letter topic.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"movie_booking/internal/usecase"
)

type Publisher struct{ w *kafkago.Writer }

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{w: &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Balancer:               &kafkago.Hash{},
		RequiredAcks:           kafkago.RequireAll, // wait for all in-sync replicas
		AllowAutoTopicCreation: true,
		WriteTimeout:           5 * time.Second,
	}}
}

var _ usecase.EventPublisher = (*Publisher)(nil)

func (p *Publisher) Publish(ctx context.Context, topic, key string, value []byte) error {
	return p.w.WriteMessages(ctx, kafkago.Message{Topic: topic, Key: []byte(key), Value: value})
}

func (p *Publisher) Close() error { return p.w.Close() }

// RetryPolicy bounds in-process retries. Exponential backoff with full jitter.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

func (rp RetryPolicy) delay(attempt int) time.Duration {
	d := rp.BaseDelay << attempt
	if d > rp.MaxDelay || d <= 0 {
		d = rp.MaxDelay
	}
	return time.Duration(rand.Int64N(int64(d) + 1))
}

// Consumer reads one topic in a consumer group. A message is committed only
// after the handler succeeds or after it is parked on the DLQ, so nothing is
// silently dropped.
type Consumer struct {
	reader   *kafkago.Reader
	dlq      usecase.EventPublisher
	dlqTopic string
	policy   RetryPolicy
	log      *slog.Logger
}

func NewConsumer(brokers []string, topic, group, dlqTopic string, dlq usecase.EventPublisher, policy RetryPolicy, log *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:     brokers,
			GroupID:     group,
			Topic:       topic,
			MinBytes:    1,
			MaxBytes:    10 << 20,
			StartOffset: kafkago.FirstOffset,
		}),
		dlq:      dlq,
		dlqTopic: dlqTopic,
		policy:   policy,
		log:      log,
	}
}

// Handler returns an error for transient failures (retried) and nil when done.
type Handler func(ctx context.Context, key, value []byte) error

func (c *Consumer) Run(ctx context.Context, handle Handler) error {
	defer c.reader.Close()
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.log.Error("fetch message failed, backing off", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
			continue
		}

		if herr := c.withRetry(ctx, msg.Key, msg.Value, handle); herr != nil {
			c.log.Error("message exhausted retries, sending to DLQ", "offset", msg.Offset, "err", herr)
			// Offsets are cumulative: skipping past a message that never reached
			// the DLQ would lose it. So block here until the DLQ accepts it.
			if !c.publishDLQ(ctx, msg) {
				return nil // shutting down; the message is not committed and will be redelivered
			}
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			c.log.Warn("commit offset failed", "err", err)
		}
	}
}

// publishDLQ retries until the DLQ accepts the message or the context ends.
func (c *Consumer) publishDLQ(ctx context.Context, msg kafkago.Message) bool {
	for attempt := 0; ; attempt++ {
		err := c.dlq.Publish(ctx, c.dlqTopic, string(msg.Key), msg.Value)
		if err == nil {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		wait := c.policy.delay(attempt)
		c.log.Error("DLQ publish failed, retrying", "wait", wait, "err", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(wait):
		}
	}
}

func (c *Consumer) withRetry(ctx context.Context, key, value []byte, handle Handler) error {
	var err error
	for attempt := 0; attempt < c.policy.MaxAttempts; attempt++ {
		if err = handle(ctx, key, value); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("interrupted: %w", ctx.Err())
		}
		if attempt+1 < c.policy.MaxAttempts {
			wait := c.policy.delay(attempt)
			c.log.Warn("handler failed, retrying", "attempt", attempt+1, "wait", wait, "err", err)
			select {
			case <-ctx.Done():
				return errors.Join(err, ctx.Err())
			case <-time.After(wait):
			}
		}
	}
	return err
}
