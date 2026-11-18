package application

import (
	"context"
	"errors"
	"fmt"

	"movie-booking/modules/booking/domain"
)

var ErrInvalidBookingConfirmation = errors.New("invalid booking confirmation")

type SendBookingConfirmationCommand struct {
	mailer Mailer
}

func NewSendBookingConfirmationCommand(mailer Mailer) *SendBookingConfirmationCommand {
	return &SendBookingConfirmationCommand{mailer: mailer}
}

func (s *SendBookingConfirmationCommand) Execute(
	ctx context.Context,
	email BookingConfirmationEmail,
) error {
	if _, err := domain.NewBookingID(email.BookingID); err != nil {
		return ErrInvalidBookingConfirmation
	}
	if _, err := domain.NewEmailAddress(email.RecipientEmail); err != nil {
		return ErrInvalidBookingConfirmation
	}
	if err := s.mailer.SendBookingConfirmation(ctx, email); err != nil {
		return fmt.Errorf("send booking confirmation email: %w", err)
	}
	return nil
}
