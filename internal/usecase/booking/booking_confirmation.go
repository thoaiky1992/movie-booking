package booking

import (
	"context"
	"errors"
	"fmt"

	"movie-booking/internal/usecase/interfaces"
)

var ErrInvalidBookingConfirmation = errors.New("invalid booking confirmation")

type BookingConfirmationService struct {
	mailer interfaces.Mailer
}

func NewBookingConfirmationService(mailer interfaces.Mailer) *BookingConfirmationService {
	return &BookingConfirmationService{mailer: mailer}
}

func (s *BookingConfirmationService) Send(
	ctx context.Context,
	email interfaces.BookingConfirmationEmail,
) error {
	if email.BookingID <= 0 || email.RecipientEmail == "" {
		return ErrInvalidBookingConfirmation
	}
	if err := s.mailer.SendBookingConfirmation(ctx, email); err != nil {
		return fmt.Errorf("send booking confirmation email: %w", err)
	}
	return nil
}
