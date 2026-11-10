package mail

import (
	"context"
	"log/slog"

	"movie_booking/internal/usecase"
)

// LogMailer is the email mock: it logs each message instead of sending it.
// The Kafka consumer, dedupe and retry path still run exactly as in production.
type LogMailer struct{ Log *slog.Logger }

var _ usecase.Mailer = LogMailer{}

func (l LogMailer) Send(_ context.Context, msg usecase.EmailMessage) error {
	l.Log.Info("email mock: message logged, not sent",
		"to", msg.To,
		"subject", msg.Subject,
		"html_bytes", len(msg.HTMLBody),
		"inline_images", len(msg.Inline),
	)
	return nil
}
