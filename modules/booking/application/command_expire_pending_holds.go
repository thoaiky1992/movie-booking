package application

import (
	"context"
	"errors"
	"fmt"

	"movie-booking/modules/booking/domain"
)

var ErrInvalidExpiryBatchSize = errors.New("expiry batch size must be positive")

type HoldLockReleaseFailure struct {
	ShowtimeID int64
	SeatCount  int
	Err        error
}

type HoldExpiryResult struct {
	ReleasedSeats       int
	DatabaseCalls       int
	LockReleaseFailures []HoldLockReleaseFailure
}

type ExpirePendingHoldsCommand struct {
	repository PendingHoldRepository
	locker     ExpiredSeatLockReleaser
}

func NewExpirePendingHoldsCommand(
	repository PendingHoldRepository,
	locker ExpiredSeatLockReleaser,
) *ExpirePendingHoldsCommand {
	return &ExpirePendingHoldsCommand{repository: repository, locker: locker}
}

func (s *ExpirePendingHoldsCommand) Execute(
	ctx context.Context,
	batchSize int,
) (HoldExpiryResult, error) {
	var result HoldExpiryResult
	if batchSize <= 0 {
		return result, ErrInvalidExpiryBatchSize
	}

	for {
		result.DatabaseCalls++
		released, err := s.repository.ExpirePendingHolds(ctx, batchSize)
		if err != nil {
			return result, fmt.Errorf("expire pending holds: %w", err)
		}
		result.ReleasedSeats += len(released)
		result.LockReleaseFailures = append(
			result.LockReleaseFailures,
			s.releaseSeatLocks(ctx, released)...,
		)
		if len(released) < batchSize {
			return result, nil
		}
	}
}

func (s *ExpirePendingHoldsCommand) releaseSeatLocks(
	ctx context.Context,
	released []domain.ReleasedSeat,
) []HoldLockReleaseFailure {
	if s.locker == nil || len(released) == 0 {
		return nil
	}

	type holdKey struct {
		showtimeID int64
		token      string
	}
	groups := make(map[holdKey][]int64)
	for _, seat := range released {
		key := holdKey{showtimeID: seat.ShowtimeID, token: seat.HoldToken}
		groups[key] = append(groups[key], seat.SeatID)
	}

	var failures []HoldLockReleaseFailure
	for key, seatIDs := range groups {
		if err := s.locker.Release(ctx, key.showtimeID, seatIDs, key.token); err != nil {
			failures = append(failures, HoldLockReleaseFailure{
				ShowtimeID: key.showtimeID,
				SeatCount:  len(seatIDs),
				Err:        err,
			})
		}
	}
	return failures
}
