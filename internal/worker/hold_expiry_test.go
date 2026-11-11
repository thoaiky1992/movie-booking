package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"movie-booking/internal/domain"
	"movie-booking/internal/observability"
)

type bookingRepositoryStub struct {
	released []domain.ReleasedSeat
	traceID  string
}

func (s *bookingRepositoryStub) HoldSeats(
	context.Context,
	int64,
	int64,
	[]int64,
	string,
	time.Duration,
) (*domain.SeatHold, error) {
	return nil, nil
}

func (s *bookingRepositoryStub) CancelBooking(
	context.Context,
	int64,
	int64,
) (*domain.CancelledBooking, error) {
	return nil, nil
}

func (s *bookingRepositoryStub) ExpirePendingHolds(
	ctx context.Context,
	_ int,
) ([]domain.ReleasedSeat, error) {
	s.traceID = observability.TraceIDFromContext(ctx)
	return s.released, nil
}

type seatLockReleaserStub struct {
	traceID string
}

func (s *seatLockReleaserStub) Release(
	ctx context.Context,
	_ int64,
	_ []int64,
	_ string,
) error {
	s.traceID = observability.TraceIDFromContext(ctx)
	return nil
}

func TestRunCycleAddsAndPropagatesTraceID(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug}))
	repository := &bookingRepositoryStub{released: []domain.ReleasedSeat{
		{ShowtimeID: 10, SeatID: 20, HoldToken: "hold-token"},
	}}
	locker := &seatLockReleaserStub{}
	w := NewHoldExpiryWorker(repository, locker, logger, time.Second, 100)

	w.runCycle(context.Background())

	entries := decodeLogEntries(t, &output)
	if len(entries) != 3 {
		t.Fatalf("log entry count = %d, want 3", len(entries))
	}
	traceID, ok := entries[0]["trace_id"].(string)
	if !ok || len(traceID) != 32 {
		t.Fatalf("trace_id = %#v, want a 32-character string", entries[0]["trace_id"])
	}
	for _, entry := range entries {
		if entry["trace_id"] != traceID {
			t.Fatalf("log trace_id = %#v, want %q", entry["trace_id"], traceID)
		}
	}
	if repository.traceID != traceID || locker.traceID != traceID {
		t.Fatalf(
			"trace propagation failed: log=%q repository=%q Redis=%q",
			traceID,
			repository.traceID,
			locker.traceID,
		)
	}
	completed := entries[len(entries)-1]
	if completed["msg"] != "seat_hold_expiry_cycle_completed" || completed["released_seats"] != float64(1) {
		t.Fatalf("unexpected completion log: %+v", completed)
	}
}

func decodeLogEntries(t *testing.T, reader io.Reader) []map[string]any {
	t.Helper()
	decoder := json.NewDecoder(reader)
	entries := make([]map[string]any, 0)
	for {
		var entry map[string]any
		if err := decoder.Decode(&entry); err == io.EOF {
			return entries
		} else if err != nil {
			t.Fatalf("decode log entry: %v", err)
		}
		entries = append(entries, entry)
	}
}
