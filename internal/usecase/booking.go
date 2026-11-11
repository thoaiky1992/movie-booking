package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"

	"movie-booking/internal/domain"
)

type SeatLocker interface {
	Acquire(
		ctx context.Context,
		showtimeID int64,
		seatIDs []int64,
		owner string,
		ttl time.Duration,
	) (bool, error)
	Release(ctx context.Context, showtimeID int64, seatIDs []int64, owner string) error
}

type BookingService struct {
	repository domain.BookingRepository
	locker     SeatLocker
	logger     *slog.Logger
	holdTTL    time.Duration
	timeout    time.Duration
}

func NewBookingService(
	repository domain.BookingRepository,
	locker SeatLocker,
	logger *slog.Logger,
	holdTTL time.Duration,
) *BookingService {
	return &BookingService{
		repository: repository,
		locker:     locker,
		logger:     logger,
		holdTTL:    holdTTL,
		timeout:    5 * time.Second,
	}
}

func (s *BookingService) HoldSeats(
	ctx context.Context,
	userID int64,
	showtimeID int64,
	seatIDs []int64,
) (*domain.SeatHold, error) {
	seatIDs, err := normalizeAndValidateHold(userID, showtimeID, seatIDs)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	holdToken := uuid.NewString()
	redisAcquired := false
	if s.locker != nil {
		// Keep Redis on the fast path. If it is unhealthy, preserve most of the
		// request budget for the authoritative PostgreSQL transaction.
		redisCtx, redisCancel := context.WithTimeout(ctx, 250*time.Millisecond)
		acquired, err := s.locker.Acquire(
			redisCtx,
			showtimeID,
			seatIDs,
			holdToken,
			s.holdTTL,
		)
		redisCancel()
		if err != nil {
			// PostgreSQL is the source of truth. A Redis outage increases database
			// contention, but must not make the correctness invariant unavailable.
			s.logger.Warn("redis_seat_lock_unavailable", "error", err)
		} else if !acquired {
			return nil, domain.ErrSeatsUnavailable
		} else {
			redisAcquired = true
		}
	}

	hold, err := s.repository.HoldSeats(
		ctx,
		userID,
		showtimeID,
		seatIDs,
		holdToken,
		s.holdTTL,
	)
	if err == nil {
		return hold, nil
	}
	if redisAcquired {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Second)
		defer releaseCancel()
		if releaseErr := s.locker.Release(
			releaseCtx,
			showtimeID,
			seatIDs,
			holdToken,
		); releaseErr != nil {
			s.logger.Warn("redis_seat_lock_rollback_failed", "error", releaseErr)
		}
	}
	return nil, fmt.Errorf("hold seats: %w", err)
}

func (s *BookingService) CancelBooking(
	ctx context.Context,
	userID, bookingID int64,
) (*domain.CancelledBooking, error) {
	if userID <= 0 || bookingID <= 0 {
		return nil, domain.ErrInvalidBookingID
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cancelled, err := s.repository.CancelBooking(ctx, userID, bookingID)
	if err != nil {
		return nil, fmt.Errorf("cancel booking: %w", err)
	}

	if s.locker != nil && cancelled.HoldToken != "" && len(cancelled.SeatIDs) > 0 {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), time.Second)
		releaseErr := s.locker.Release(
			releaseCtx,
			cancelled.ShowtimeID,
			cancelled.SeatIDs,
			cancelled.HoldToken,
		)
		releaseCancel()
		if releaseErr != nil {
			// PostgreSQL is authoritative; Redis TTL still bounds a stale lock.
			s.logger.Warn("redis_cancelled_hold_release_failed", "error", releaseErr)
		}
	}
	return cancelled, nil
}

func normalizeAndValidateHold(userID, showtimeID int64, seatIDs []int64) ([]int64, error) {
	if userID <= 0 || showtimeID <= 0 || len(seatIDs) == 0 ||
		len(seatIDs) > domain.MaxSeatsPerHold {
		return nil, domain.ErrInvalidSeatHold
	}
	normalizedSeatIDs := append([]int64(nil), seatIDs...)
	sort.Slice(normalizedSeatIDs, func(i, j int) bool {
		return normalizedSeatIDs[i] < normalizedSeatIDs[j]
	})
	for i, seatID := range normalizedSeatIDs {
		if seatID <= 0 || (i > 0 && seatID == normalizedSeatIDs[i-1]) {
			return nil, domain.ErrInvalidSeatHold
		}
	}
	return normalizedSeatIDs, nil
}
