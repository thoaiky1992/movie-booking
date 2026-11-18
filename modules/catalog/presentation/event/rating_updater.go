package event

import (
	"context"
	"log/slog"
	"time"

	catalogapplication "movie-booking/modules/catalog/application"
	"movie-booking/shared/observability"
)

type RatingUpdater struct {
	useCase  *catalogapplication.UpdateExternalRatingCommand
	logger   *slog.Logger
	interval time.Duration
}

func NewRatingUpdater(
	useCase *catalogapplication.UpdateExternalRatingCommand,
	logger *slog.Logger,
	interval time.Duration,
) *RatingUpdater {
	return &RatingUpdater{
		useCase:  useCase,
		logger:   logger,
		interval: interval,
	}
}

func (w *RatingUpdater) Run(ctx context.Context) {
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

func (w *RatingUpdater) runCycle(parentCtx context.Context) {
	traceID := observability.NewTraceID()
	ctx := observability.ContextWithTraceID(parentCtx, traceID)
	logger := w.logger.With("trace_id", traceID)
	startedAt := time.Now()

	updated, err := w.useCase.Execute(ctx)
	if err != nil {
		if ctx.Err() == nil {
			logger.ErrorContext(
				ctx,
				"external_rating_update_failed",
				"latency_ms", time.Since(startedAt).Milliseconds(),
				"error", err,
			)
		}
		return
	}

	log := logger.DebugContext
	if updated > 0 {
		log = logger.InfoContext
	}
	log(
		ctx,
		"external_rating_update_completed",
		"updated_ratings", updated,
		"latency_ms", time.Since(startedAt).Milliseconds(),
	)
}
