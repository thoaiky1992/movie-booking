package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewBookingReference(t *testing.T) {
	t.Parallel()

	reference, err := NewBookingReference(7, 42)
	if err != nil {
		t.Fatalf("NewBookingReference() error = %v", err)
	}
	if reference.UserID().Int64() != 7 || reference.BookingID().Int64() != 42 {
		t.Fatalf("NewBookingReference() = user %d, booking %d", reference.UserID().Int64(), reference.BookingID().Int64())
	}

	for _, test := range []struct {
		name      string
		userID    int64
		bookingID int64
	}{
		{name: "invalid user", userID: 0, bookingID: 42},
		{name: "invalid booking", userID: 7, bookingID: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewBookingReference(test.userID, test.bookingID)
			if !errors.Is(err, ErrInvalidBookingID) {
				t.Fatalf("NewBookingReference() error = %v, want %v", err, ErrInvalidBookingID)
			}
		})
	}
}

func TestNewSeatHoldRequest(t *testing.T) {
	t.Parallel()

	rawSeats := []int64{3, 1, 2}
	request, err := NewSeatHoldRequest(7, 9, rawSeats)
	if err != nil {
		t.Fatalf("NewSeatHoldRequest() error = %v", err)
	}
	if got := SeatIDValues(request.SeatIDs()); !reflect.DeepEqual(got, []int64{1, 2, 3}) {
		t.Fatalf("SeatIDs() = %v, want [1 2 3]", got)
	}
	if !reflect.DeepEqual(rawSeats, []int64{3, 1, 2}) {
		t.Fatalf("constructor mutated input: %v", rawSeats)
	}

	invalidRequests := []struct {
		name       string
		userID     int64
		showtimeID int64
		seatIDs    []int64
	}{
		{name: "invalid user", userID: 0, showtimeID: 9, seatIDs: []int64{1}},
		{name: "invalid showtime", userID: 7, showtimeID: 0, seatIDs: []int64{1}},
		{name: "empty seats", userID: 7, showtimeID: 9},
		{name: "invalid seat", userID: 7, showtimeID: 9, seatIDs: []int64{0}},
		{name: "duplicate seat", userID: 7, showtimeID: 9, seatIDs: []int64{1, 1}},
	}
	for _, test := range invalidRequests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSeatHoldRequest(test.userID, test.showtimeID, test.seatIDs)
			if !errors.Is(err, ErrInvalidSeatHold) {
				t.Fatalf("NewSeatHoldRequest() error = %v, want %v", err, ErrInvalidSeatHold)
			}
		})
	}
}
