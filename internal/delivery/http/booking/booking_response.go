package booking

import (
	"time"

	"movie-booking/internal/domain"
)

type seatHoldResponse struct {
	BookingID   int64     `json:"booking_id"`
	ShowtimeID  int64     `json:"showtime_id"`
	SeatIDs     []int64   `json:"seat_ids"`
	HoldToken   string    `json:"hold_token"`
	Status      string    `json:"status"`
	TotalAmount string    `json:"total_amount"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type confirmedBookingResponse struct {
	BookingID   int64     `json:"booking_id"`
	ShowtimeID  int64     `json:"showtime_id"`
	SeatIDs     []int64   `json:"seat_ids"`
	Status      string    `json:"status"`
	TotalAmount string    `json:"total_amount"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

type cancelledBookingResponse struct {
	BookingID  int64   `json:"booking_id"`
	ShowtimeID int64   `json:"showtime_id"`
	SeatIDs    []int64 `json:"seat_ids"`
	Status     string  `json:"status"`
}

func newSeatHoldResponse(hold *domain.SeatHold) *seatHoldResponse {
	if hold == nil {
		return nil
	}
	return &seatHoldResponse{
		BookingID:   hold.BookingID,
		ShowtimeID:  hold.ShowtimeID,
		SeatIDs:     hold.SeatIDs,
		HoldToken:   hold.HoldToken,
		Status:      hold.Status,
		TotalAmount: hold.TotalAmount,
		ExpiresAt:   hold.ExpiresAt,
	}
}

func newConfirmedBookingResponse(booking *domain.ConfirmedBooking) *confirmedBookingResponse {
	if booking == nil {
		return nil
	}
	return &confirmedBookingResponse{
		BookingID:   booking.BookingID,
		ShowtimeID:  booking.ShowtimeID,
		SeatIDs:     booking.SeatIDs,
		Status:      booking.Status,
		TotalAmount: booking.TotalAmount,
		ConfirmedAt: booking.ConfirmedAt,
	}
}

func newCancelledBookingResponse(booking *domain.CancelledBooking) *cancelledBookingResponse {
	if booking == nil {
		return nil
	}
	return &cancelledBookingResponse{
		BookingID:  booking.BookingID,
		ShowtimeID: booking.ShowtimeID,
		SeatIDs:    booking.SeatIDs,
		Status:     booking.Status,
	}
}
