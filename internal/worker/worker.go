// Package worker runs the periodic jobs that the API must not run on the request path:
// outbox relay, hold expiry, refund retries and partner rating sync.
package worker

import (
	"context"
	"log/slog"
	"time"
)

// Every runs job immediately and then once per interval until ctx is cancelled.
// A failing run is logged and the schedule continues, so one bad batch never stops a job.
func Every(ctx context.Context, log *slog.Logger, name string, interval time.Duration, job func(ctx context.Context) error) {
	run := func() {
		start := time.Now()
		if err := job(ctx); err != nil && ctx.Err() == nil {
			log.Error("job failed", "job", name, "err", err)
		}
		if d := time.Since(start); d > interval {
			log.Warn("job run exceeded its interval", "job", name, "took", d, "interval", interval)
		}
	}

	run()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
