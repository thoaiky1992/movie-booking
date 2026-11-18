package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"movie-booking/modules/booking/domain"
)

type HoldSeatsCommand struct {
	repository BookingRepository
	locker     SeatLocker
	logger     Logger
	holdTTL    time.Duration
	timeout    time.Duration
}

func NewHoldSeatsCommand(
	repository BookingRepository,
	locker SeatLocker,
	logger Logger,
	holdTTL time.Duration,
) *HoldSeatsCommand {
	return &HoldSeatsCommand{
		repository: repository, locker: locker, logger: loggerOrDiscard(logger),
		holdTTL: holdTTL, timeout: 5 * time.Second,
	}
}

func (c *HoldSeatsCommand) Execute(
	ctx context.Context,
	userID, showtimeID int64,
	seatIDs []int64,
) (*domain.SeatHold, error) {
	request, err := domain.NewSeatHoldRequest(userID, showtimeID, seatIDs)
	if err != nil {
		return nil, err
	}
	seatIDValues := domain.SeatIDValues(request.SeatIDs())
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	holdToken := uuid.NewString()
	redisAcquired := false
	if c.locker != nil {
		redisCtx, redisCancel := context.WithTimeout(ctx, 250*time.Millisecond)
		acquired, lockErr := c.locker.Acquire(
			redisCtx, request.ShowtimeID().Int64(), seatIDValues, holdToken, c.holdTTL,
		)
		redisCancel()
		if lockErr != nil {
			c.logger.WarnContext(ctx, "redis_seat_lock_unavailable", "error", lockErr)
		} else if !acquired {
			return nil, domain.ErrSeatsUnavailable
		} else {
			redisAcquired = true
		}
	}

	hold, err := c.repository.HoldSeats(
		ctx, request.UserID(), request.ShowtimeID(), request.SeatIDs(), holdToken, c.holdTTL,
	)
	if err == nil {
		return hold, nil
	}
	if redisAcquired {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Second)
		defer releaseCancel()
		if releaseErr := c.locker.Release(
			releaseCtx, request.ShowtimeID().Int64(), seatIDValues, holdToken,
		); releaseErr != nil {
			c.logger.WarnContext(ctx, "redis_seat_lock_rollback_failed", "error", releaseErr)
		}
	}
	return nil, fmt.Errorf("hold seats: %w", err)
}
