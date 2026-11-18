package application

import (
	"context"
	"fmt"
	"time"

	"movie-booking/modules/booking/domain"
)

type CancelBookingCommand struct {
	repository BookingRepository
	locker     SeatLocker
	logger     Logger
	timeout    time.Duration
}

func NewCancelBookingCommand(
	repository BookingRepository,
	locker SeatLocker,
	logger Logger,
) *CancelBookingCommand {
	return &CancelBookingCommand{
		repository: repository, locker: locker,
		logger: loggerOrDiscard(logger), timeout: 5 * time.Second,
	}
}

func (c *CancelBookingCommand) Execute(
	ctx context.Context,
	userID, bookingID int64,
) (*domain.CancelledBooking, error) {
	reference, err := domain.NewBookingReference(userID, bookingID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	cancelled, err := c.repository.CancelBooking(ctx, reference.UserID(), reference.BookingID())
	if err != nil {
		return nil, fmt.Errorf("cancel booking: %w", err)
	}
	if c.locker != nil && cancelled.HoldToken != "" && len(cancelled.SeatIDs) > 0 {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Second)
		releaseErr := c.locker.Release(releaseCtx, cancelled.ShowtimeID, cancelled.SeatIDs, cancelled.HoldToken)
		releaseCancel()
		if releaseErr != nil {
			c.logger.WarnContext(ctx, "redis_cancelled_hold_release_failed", "error", releaseErr)
		}
	}
	return cancelled, nil
}
