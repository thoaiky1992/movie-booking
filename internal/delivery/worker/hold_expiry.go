package worker

import (
	"context"
	"log/slog"
	"time"

	"movie-booking/internal/infrastructure/observability"
	bookingusecase "movie-booking/internal/usecase/booking"
)

type HoldExpiryUseCase interface {
	ExpirePendingHolds(ctx context.Context, batchSize int) (bookingusecase.HoldExpiryResult, error)
}

type HoldExpiryWorker struct {
	useCase   HoldExpiryUseCase
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
}

func NewHoldExpiryWorker(
	useCase HoldExpiryUseCase,
	logger *slog.Logger,
	interval time.Duration,
	batchSize int,
) *HoldExpiryWorker {
	return &HoldExpiryWorker{
		useCase:   useCase,
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
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
	logger.DebugContext(ctx, "seat_hold_expiry_cycle_started")
	result, err := w.useCase.ExpirePendingHolds(ctx, w.batchSize)
	for _, failure := range result.LockReleaseFailures {
		logger.WarnContext(
			ctx,
			"expired_redis_seat_lock_release_failed",
			"showtime_id", failure.ShowtimeID,
			"seat_count", failure.SeatCount,
			"error", failure.Err,
		)
	}
	if err != nil {
		if ctx.Err() == nil {
			logger.ErrorContext(
				ctx,
				"seat_hold_expiry_failed",
				"database_calls", result.DatabaseCalls,
				"released_seats", result.ReleasedSeats,
				"latency_ms", time.Since(startedAt).Milliseconds(),
				"error", err,
			)
		}
		return
	}

	log := logger.DebugContext
	if result.ReleasedSeats > 0 {
		log = logger.InfoContext
	}
	log(
		ctx,
		"seat_hold_expiry_cycle_completed",
		"database_calls", result.DatabaseCalls,
		"released_seats", result.ReleasedSeats,
		"latency_ms", time.Since(startedAt).Milliseconds(),
	)
}
