package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"movie-booking/internal/domain"
)

type bookingRepositoryStub struct {
	hold       *domain.SeatHold
	err        error
	userID     int64
	showtimeID int64
	seatIDs    []int64
	holdToken  string
	ttl        time.Duration
	called     bool
}

func (s *bookingRepositoryStub) HoldSeats(
	_ context.Context,
	userID int64,
	showtimeID int64,
	seatIDs []int64,
	holdToken string,
	ttl time.Duration,
) (*domain.SeatHold, error) {
	s.called = true
	s.userID = userID
	s.showtimeID = showtimeID
	s.seatIDs = append([]int64(nil), seatIDs...)
	s.holdToken = holdToken
	s.ttl = ttl
	return s.hold, s.err
}

func (s *bookingRepositoryStub) ExpirePendingHolds(
	context.Context,
	int,
) ([]domain.ReleasedSeat, error) {
	return nil, nil
}

type seatLockerStub struct {
	acquired      bool
	acquireErr    error
	releaseErr    error
	releaseCalled bool
	acquiredSeats []int64
	releasedSeats []int64
	acquiredOwner string
	releasedOwner string
}

func (s *seatLockerStub) Acquire(
	_ context.Context,
	_ int64,
	seatIDs []int64,
	owner string,
	_ time.Duration,
) (bool, error) {
	s.acquiredSeats = append([]int64(nil), seatIDs...)
	s.acquiredOwner = owner
	return s.acquired, s.acquireErr
}

func (s *seatLockerStub) Release(
	_ context.Context,
	_ int64,
	seatIDs []int64,
	owner string,
) error {
	s.releaseCalled = true
	s.releasedSeats = append([]int64(nil), seatIDs...)
	s.releasedOwner = owner
	return s.releaseErr
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHoldSeatsSortsAndPersistsAllSeats(t *testing.T) {
	repository := &bookingRepositoryStub{hold: &domain.SeatHold{BookingID: 7}}
	locker := &seatLockerStub{acquired: true}
	service := NewBookingService(repository, locker, testLogger(), 15*time.Minute)

	inputSeats := []int64{9, 4, 7}
	hold, err := service.HoldSeats(context.Background(), 2, 3, inputSeats)
	if err != nil {
		t.Fatalf("HoldSeats() error = %v", err)
	}
	if hold.BookingID != 7 {
		t.Fatalf("booking ID = %d, want 7", hold.BookingID)
	}
	wantSeats := []int64{4, 7, 9}
	if !reflect.DeepEqual(repository.seatIDs, wantSeats) {
		t.Fatalf("repository seats = %v, want %v", repository.seatIDs, wantSeats)
	}
	if !reflect.DeepEqual(locker.acquiredSeats, wantSeats) {
		t.Fatalf("Redis seats = %v, want %v", locker.acquiredSeats, wantSeats)
	}
	if !reflect.DeepEqual(inputSeats, []int64{9, 4, 7}) {
		t.Fatalf("caller input was mutated: %v", inputSeats)
	}
	if repository.holdToken == "" || repository.holdToken != locker.acquiredOwner {
		t.Fatalf("hold tokens do not match: repository=%q Redis=%q", repository.holdToken, locker.acquiredOwner)
	}
}

func TestHoldSeatsStopsOnRedisConflict(t *testing.T) {
	repository := &bookingRepositoryStub{}
	locker := &seatLockerStub{acquired: false}
	service := NewBookingService(repository, locker, testLogger(), time.Minute)

	_, err := service.HoldSeats(context.Background(), 2, 3, []int64{4})
	if !errors.Is(err, domain.ErrSeatsUnavailable) {
		t.Fatalf("error = %v, want ErrSeatsUnavailable", err)
	}
	if repository.called {
		t.Fatal("repository was called after Redis reported a live hold")
	}
}

func TestHoldSeatsReleasesRedisWhenDatabaseRejects(t *testing.T) {
	repository := &bookingRepositoryStub{err: domain.ErrSeatsUnavailable}
	locker := &seatLockerStub{acquired: true}
	service := NewBookingService(repository, locker, testLogger(), time.Minute)

	_, err := service.HoldSeats(context.Background(), 2, 3, []int64{4, 5})
	if !errors.Is(err, domain.ErrSeatsUnavailable) {
		t.Fatalf("error = %v, want ErrSeatsUnavailable", err)
	}
	if !locker.releaseCalled || locker.releasedOwner != locker.acquiredOwner {
		t.Fatalf("Redis rollback not called with the owner token")
	}
}

func TestHoldSeatsFallsBackToDatabaseWhenRedisIsDown(t *testing.T) {
	repository := &bookingRepositoryStub{hold: &domain.SeatHold{BookingID: 8}}
	locker := &seatLockerStub{acquireErr: errors.New("redis unavailable")}
	service := NewBookingService(repository, locker, testLogger(), time.Minute)

	_, err := service.HoldSeats(context.Background(), 2, 3, []int64{4})
	if err != nil {
		t.Fatalf("HoldSeats() error = %v", err)
	}
	if !repository.called {
		t.Fatal("database fallback was not called")
	}
}

func TestHoldSeatsRejectsDuplicateSeats(t *testing.T) {
	service := NewBookingService(&bookingRepositoryStub{}, nil, testLogger(), time.Minute)
	_, err := service.HoldSeats(context.Background(), 2, 3, []int64{4, 4})
	if !errors.Is(err, domain.ErrInvalidSeatHold) {
		t.Fatalf("error = %v, want ErrInvalidSeatHold", err)
	}
}
