package worker

import (
	"context"
	"log/slog"
	"time"

	"movie-booking/internal/domain"
	"movie-booking/internal/observability"
)

type SeatLockReleaser interface {
	Release(ctx context.Context, showtimeID int64, seatIDs []int64, owner string) error
}

type HoldExpiryWorker struct {
	repository domain.BookingRepository
	locker     SeatLockReleaser
	logger     *slog.Logger
	interval   time.Duration
	batchSize  int
}

func NewHoldExpiryWorker(
	repository domain.BookingRepository,
	locker SeatLockReleaser,
	logger *slog.Logger,
	interval time.Duration,
	batchSize int,
) *HoldExpiryWorker {
	return &HoldExpiryWorker{
		repository: repository,
		locker:     locker,
		logger:     logger,
		interval:   interval,
		batchSize:  batchSize,
	}
}

func (w *HoldExpiryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		w.runCycle(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *HoldExpiryWorker) runCycle(parentCtx context.Context) {
	traceID := observability.NewTraceID()
	ctx := observability.ContextWithTraceID(parentCtx, traceID)
	logger := w.logger.With("trace_id", traceID)
	startedAt := time.Now()
	totalReleased := 0
	databaseCalls := 0

	logger.DebugContext(ctx, "seat_hold_expiry_cycle_started")
	for {
		databaseCalls++
		released, err := w.repository.ExpirePendingHolds(ctx, w.batchSize)
		if err != nil {
			if ctx.Err() == nil {
				logger.ErrorContext(
					ctx,
					"seat_hold_expiry_failed",
					"database_calls", databaseCalls,
					"released_seats", totalReleased,
					"latency_ms", time.Since(startedAt).Milliseconds(),
					"error", err,
				)
			}
			return
		}
		totalReleased += len(released)
		logger.DebugContext(
			ctx,
			"seat_hold_expiry_batch_completed",
			"database_call", databaseCalls,
			"released_seats", len(released),
		)
		w.releaseRedis(ctx, released, logger)
		if len(released) < w.batchSize {
			log := logger.DebugContext
			if totalReleased > 0 {
				log = logger.InfoContext
			}
			log(
				ctx,
				"seat_hold_expiry_cycle_completed",
				"database_calls", databaseCalls,
				"released_seats", totalReleased,
				"latency_ms", time.Since(startedAt).Milliseconds(),
			)
			return
		}
	}
}

func (w *HoldExpiryWorker) releaseRedis(
	ctx context.Context,
	released []domain.ReleasedSeat,
	logger *slog.Logger,
) {
	if w.locker == nil || len(released) == 0 {
		return
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
	for key, seatIDs := range groups {
		if err := w.locker.Release(ctx, key.showtimeID, seatIDs, key.token); err != nil {
			// Redis keys already have a TTL; failure here is self-healing.
			logger.WarnContext(
				ctx,
				"expired_redis_seat_lock_release_failed",
				"showtime_id", key.showtimeID,
				"seat_count", len(seatIDs),
				"error", err,
			)
		}
	}
}
