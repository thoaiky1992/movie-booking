package domain

import "errors"

var (
	ErrInvalidUserID     = errors.New("invalid user ID")
	ErrInvalidShowtimeID = errors.New("invalid showtime ID")
	ErrInvalidSeatID     = errors.New("invalid seat ID")
)

type UserID struct{ value int64 }

func NewUserID(value int64) (UserID, error) {
	if value <= 0 {
		return UserID{}, ErrInvalidUserID
	}
	return UserID{value: value}, nil
}

func (id UserID) Int64() int64 { return id.value }

type BookingID struct{ value int64 }

func NewBookingID(value int64) (BookingID, error) {
	if value <= 0 {
		return BookingID{}, ErrInvalidBookingID
	}
	return BookingID{value: value}, nil
}

func (id BookingID) Int64() int64 { return id.value }

type ShowtimeID struct{ value int64 }

func NewShowtimeID(value int64) (ShowtimeID, error) {
	if value <= 0 {
		return ShowtimeID{}, ErrInvalidShowtimeID
	}
	return ShowtimeID{value: value}, nil
}

func (id ShowtimeID) Int64() int64 { return id.value }

type SeatID struct{ value int64 }

func NewSeatID(value int64) (SeatID, error) {
	if value <= 0 {
		return SeatID{}, ErrInvalidSeatID
	}
	return SeatID{value: value}, nil
}

func (id SeatID) Int64() int64 { return id.value }

func SeatIDValues(ids []SeatID) []int64 {
	values := make([]int64, len(ids))
	for index, id := range ids {
		values[index] = id.Int64()
	}
	return values
}
