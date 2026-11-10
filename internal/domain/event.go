package domain

import "time"

// Event types published on the booking events topic.
const (
	EventBookingConfirmed = "booking.confirmed"
	EventBookingCancelled = "booking.cancelled"
)

// BookingEvent is the self-contained payload sent through the outbox to Kafka.
// It carries everything a consumer needs (recipient, movie, seats) so that
// consumers never have to call back into the booking service.
type BookingEvent struct {
	EventID     string    `json:"event_id"` // also the idempotency key for consumers
	Type        string    `json:"type"`
	BookingID   string    `json:"booking_id"`
	OccurredAt  time.Time `json:"occurred_at"`
	TicketCode  string    `json:"ticket_code"`
	Recipient   Recipient `json:"recipient"`
	MovieTitle  string    `json:"movie_title"`
	CinemaName  string    `json:"cinema_name"`
	HallName    string    `json:"hall_name"`
	StartsAt    time.Time `json:"starts_at"`
	Seats       []string  `json:"seats"`
	TotalCents  int64     `json:"total_cents"`
	RefundCents int64     `json:"refund_cents"`
	Currency    string    `json:"currency"`
}

type Recipient struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}
