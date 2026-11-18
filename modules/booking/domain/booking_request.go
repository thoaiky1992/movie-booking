package domain

import "sort"

// BookingReference is the validated identity required to mutate a booking.
type BookingReference struct {
	userID    UserID
	bookingID BookingID
}

func NewBookingReference(rawUserID, rawBookingID int64) (BookingReference, error) {
	userID, err := NewUserID(rawUserID)
	if err != nil {
		return BookingReference{}, ErrInvalidBookingID
	}
	bookingID, err := NewBookingID(rawBookingID)
	if err != nil {
		return BookingReference{}, err
	}
	return BookingReference{userID: userID, bookingID: bookingID}, nil
}

func (r BookingReference) UserID() UserID       { return r.userID }
func (r BookingReference) BookingID() BookingID { return r.bookingID }

// SeatHoldRequest guarantees that a hold has a valid owner, showtime and a
// sorted, unique collection of no more than MaxSeatsPerHold seats.
type SeatHoldRequest struct {
	userID     UserID
	showtimeID ShowtimeID
	seatIDs    []SeatID
}

func NewSeatHoldRequest(
	rawUserID, rawShowtimeID int64,
	rawSeatIDs []int64,
) (SeatHoldRequest, error) {
	userID, err := NewUserID(rawUserID)
	if err != nil {
		return SeatHoldRequest{}, ErrInvalidSeatHold
	}
	showtimeID, err := NewShowtimeID(rawShowtimeID)
	if err != nil || len(rawSeatIDs) == 0 || len(rawSeatIDs) > MaxSeatsPerHold {
		return SeatHoldRequest{}, ErrInvalidSeatHold
	}

	values := append([]int64(nil), rawSeatIDs...)
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	seatIDs := make([]SeatID, len(values))
	for index, value := range values {
		seatID, seatErr := NewSeatID(value)
		if seatErr != nil || (index > 0 && value == values[index-1]) {
			return SeatHoldRequest{}, ErrInvalidSeatHold
		}
		seatIDs[index] = seatID
	}
	return SeatHoldRequest{
		userID: userID, showtimeID: showtimeID, seatIDs: seatIDs,
	}, nil
}

func (r SeatHoldRequest) UserID() UserID         { return r.userID }
func (r SeatHoldRequest) ShowtimeID() ShowtimeID { return r.showtimeID }

func (r SeatHoldRequest) SeatIDs() []SeatID {
	return append([]SeatID(nil), r.seatIDs...)
}
