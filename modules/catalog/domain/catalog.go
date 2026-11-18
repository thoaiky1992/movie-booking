package domain

import (
	"errors"
	"time"
)

var (
	ErrInvalidCinemaID   = errors.New("invalid cinema ID")
	ErrInvalidMovieID    = errors.New("invalid movie ID")
	ErrInvalidShowtimeID = errors.New("invalid showtime ID")
	ErrMovieNotFound     = errors.New("movie not found")
	ErrShowtimeNotFound  = errors.New("showtime not found")
)

type Cinema struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type Movie struct {
	ID              int64
	Title           string
	DurationMinutes int
	CreatedAt       time.Time
}

type MovieDetail struct {
	ID               int64
	Title            string
	DurationMinutes  int
	CreatedAt        time.Time
	Synopsis         *string
	Director         *string
	CastMembers      *string
	Genres           *string
	PosterURL        *string
	TrailerURL       *string
	ReleaseDate      *string
	AgeRating        *string
	OriginalLanguage *string
	UpdatedAt        *time.Time
	ExternalRatings  []MovieExternalRating
}

type MovieExternalRating struct {
	ID           int64
	Source       string
	RatingScore  float64
	MaxScore     float64
	VoteCount    *int64
	ExternalURL  *string
	LastSyncedAt time.Time
}

type Showtime struct {
	ID       int64
	StartsAt time.Time
	EndsAt   time.Time
	Movie    MovieInfo
	Cinema   PlaceInfo
	Room     PlaceInfo
}

type MovieInfo struct {
	ID              int64
	Title           string
	DurationMinutes int
}

type PlaceInfo struct {
	ID   int64
	Name string
}

type SeatStatus string

const (
	SeatAvailable SeatStatus = "AVAILABLE"
	SeatHeld      SeatStatus = "HELD"
	SeatBooked    SeatStatus = "BOOKED"
)

type Seat struct {
	ID     int64
	Number int
	Status SeatStatus
}

type SeatRow struct {
	Label string
	Seats []Seat
}

type SeatMap struct {
	ShowtimeID int64
	Movie      MovieInfo
	Cinema     PlaceInfo
	Room       PlaceInfo
	StartsAt   time.Time
	EndsAt     time.Time
	Rows       []SeatRow
	Summary    SeatSummary
}

type SeatSummary struct {
	Total     int
	Available int
	Held      int
	Booked    int
}
