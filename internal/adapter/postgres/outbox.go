package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"movie_booking/internal/usecase"
)

// OutboxRelay publishes committed outbox rows to the message broker.
//
// Delivery is at-least-once: a row is marked published only after the broker
// acknowledges it, so a crash can cause a duplicate, never a loss. Consumers
// de-duplicate on event_id.
type OutboxRelay struct {
	pool      *pgxpool.Pool
	publisher usecase.EventPublisher
	batchSize int
}

func NewOutboxRelay(pool *pgxpool.Pool, publisher usecase.EventPublisher, batchSize int) *OutboxRelay {
	return &OutboxRelay{pool: pool, publisher: publisher, batchSize: batchSize}
}

type outboxRow struct {
	id, topic, key string
	payload        []byte
}

// PublishPending publishes up to batchSize rows. SKIP LOCKED allows several
// relays to run side by side without publishing the same row concurrently.
func (r *OutboxRelay) PublishPending(ctx context.Context) (int, error) {
	published := 0
	err := withTx(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id::text, topic, message_key, payload::text
			FROM outbox_events
			WHERE published_at IS NULL
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, r.batchSize)
		if err != nil {
			return fmt.Errorf("select outbox: %w", err)
		}
		var batch []outboxRow
		for rows.Next() {
			var row outboxRow
			var payload string
			if err := rows.Scan(&row.id, &row.topic, &row.key, &payload); err != nil {
				rows.Close()
				return fmt.Errorf("scan outbox row: %w", err)
			}
			row.payload = []byte(payload)
			batch = append(batch, row)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}

		ids := make([]string, 0, len(batch))
		for _, row := range batch {
			if err := r.publisher.Publish(ctx, row.topic, row.key, row.payload); err != nil {
				return fmt.Errorf("publish outbox %s: %w", row.id, err)
			}
			ids = append(ids, row.id)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE outbox_events SET published_at = now() WHERE id = ANY($1::uuid[])`, ids); err != nil {
			return fmt.Errorf("mark outbox published: %w", err)
		}
		published = len(ids)
		return nil
	})
	return published, err
}
