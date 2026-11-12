package worker

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"movie-booking/internal/observability"
)

type externalRatingRandomizerStub struct {
	updated int64
	traceID string
}

func (s *externalRatingRandomizerStub) RandomizeOne(ctx context.Context) (int64, error) {
	s.traceID = observability.TraceIDFromContext(ctx)
	return s.updated, nil
}

func TestRatingUpdaterRunCycleUpdatesRatingWithTraceID(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	repository := &externalRatingRandomizerStub{updated: 1}
	w := NewRatingUpdater(repository, logger, time.Second)

	w.runCycle(context.Background())

	entries := decodeLogEntries(t, &output)
	if len(entries) != 1 {
		t.Fatalf("log entry count = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry["msg"] != "external_rating_update_completed" || entry["updated_ratings"] != float64(1) {
		t.Fatalf("unexpected completion log: %+v", entry)
	}
	traceID, ok := entry["trace_id"].(string)
	if !ok || len(traceID) != 32 {
		t.Fatalf("trace_id = %#v, want a 32-character string", entry["trace_id"])
	}
	if repository.traceID != traceID {
		t.Fatalf("repository trace_id = %q, want %q", repository.traceID, traceID)
	}
}
