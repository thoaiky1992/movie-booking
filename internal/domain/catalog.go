package domain

import (
	"time"

	"github.com/google/uuid"
)

type Movie struct {
	ID              uuid.UUID
	Title           string
	Description     string
	DurationMinutes int
	Genres          []string
	Certificate     string
	ReleaseDate     *time.Time
	PosterURL       string
	IMDbID          *string // e.g. "tt0111161"
	TMDbID          *int64
}

type Cinema struct {
	ID      uuid.UUID
	Name    string
	Address string
	City    string
}

type Hall struct {
	ID       uuid.UUID
	CinemaID uuid.UUID
	Name     string
}

type Seat struct {
	ID                 uuid.UUID
	HallID             uuid.UUID
	RowLabel           string
	SeatNumber         int
	SeatType           string
	PriceModifierCents int64
}

type Showtime struct {
	ID             uuid.UUID
	MovieID        uuid.UUID
	HallID         uuid.UUID
	StartsAt       time.Time
	EndsAt         time.Time
	BasePriceCents int64
}

// ShowtimeView is the denormalised listing row returned by the showtime search.
type ShowtimeView struct {
	Showtime
	MovieTitle string
	CinemaID   uuid.UUID
	CinemaName string
	HallName   string
}

// SeatStatus values for the seat map.
const (
	SeatAvailable = "AVAILABLE"
	SeatHeld      = "HELD"
	SeatSold      = "SOLD"
)

type SeatStatus struct {
	Seat
	Status     string
	PriceCents int64
}
