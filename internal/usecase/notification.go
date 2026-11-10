package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"movie_booking/internal/domain"
)

// NotificationService turns booking events into emails. It is idempotent:
// a redelivered event never produces a second email.
type NotificationService struct {
	dedupe   Dedupe
	composer TicketComposer
	mailer   Mailer
	ttl      time.Duration
	log      *slog.Logger
}

func NewNotificationService(dedupe Dedupe, composer TicketComposer, mailer Mailer, ttl time.Duration, log *slog.Logger) *NotificationService {
	return &NotificationService{dedupe: dedupe, composer: composer, mailer: mailer, ttl: ttl, log: log}
}

// Handle returns an error only for transient failures, which the consumer retries.
func (n *NotificationService) Handle(ctx context.Context, evt domain.BookingEvent) error {
	switch evt.Type {
	case domain.EventBookingConfirmed, domain.EventBookingCancelled:
	default:
		return nil // unknown types are acknowledged and ignored
	}

	key := "notif:sent:" + evt.EventID
	claimed, err := n.dedupe.Claim(ctx, key, n.ttl)
	if err != nil {
		return fmt.Errorf("claim notification: %w", err)
	}
	if !claimed {
		n.log.Info("duplicate booking event skipped", "event_id", evt.EventID)
		return nil
	}

	msg, err := n.composer.Compose(evt)
	if err != nil {
		_ = n.dedupe.Release(ctx, key)
		return fmt.Errorf("compose email: %w", err)
	}
	if err := n.mailer.Send(ctx, msg); err != nil {
		// Release the claim so the retry can actually send.
		if rErr := n.dedupe.Release(ctx, key); rErr != nil {
			n.log.Warn("release notification claim failed", "event_id", evt.EventID, "err", rErr)
		}
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
