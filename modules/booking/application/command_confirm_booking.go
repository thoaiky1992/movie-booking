package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/booking/domain"
)

type ConfirmBookingCommand struct {
	repository BookingRepository
	locker     SeatLocker
	enqueuer   BookingConfirmationEnqueuer
	logger     Logger
	timeout    time.Duration
}

func NewConfirmBookingCommand(
	repository BookingRepository,
	locker SeatLocker,
	enqueuer BookingConfirmationEnqueuer,
	logger Logger,
) *ConfirmBookingCommand {
	return &ConfirmBookingCommand{
		repository: repository, locker: locker, enqueuer: enqueuer,
		logger: loggerOrDiscard(logger), timeout: 5 * time.Second,
	}
}

func (c *ConfirmBookingCommand) Execute(
	ctx context.Context,
	userID, bookingID int64,
) (*domain.ConfirmedBooking, error) {
	reference, err := domain.NewBookingReference(userID, bookingID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	confirmed, err := c.repository.ConfirmBooking(ctx, reference.UserID(), reference.BookingID())
	if err != nil {
		return nil, fmt.Errorf("confirm booking: %w", err)
	}
	if c.enqueuer != nil {
		if err := c.enqueuer.EnqueueBookingConfirmation(ctx, confirmed); err != nil {
			c.logger.ErrorContext(ctx, "booking_confirmation_enqueue_failed", "booking_id", confirmed.BookingID, "error", err)
		}
	}
	if c.locker != nil && confirmed.HoldToken != "" && len(confirmed.SeatIDs) > 0 {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Second)
		releaseErr := c.locker.Release(releaseCtx, confirmed.ShowtimeID, confirmed.SeatIDs, confirmed.HoldToken)
		releaseCancel()
		if releaseErr != nil {
			c.logger.WarnContext(ctx, "redis_confirmed_hold_release_failed", "error", releaseErr)
		}
	}
	return confirmed, nil
}
