package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrMovieNotFound    = errors.New("movie not found")
	ErrShowtimeNotFound = errors.New("showtime not found")
)

type Cinema struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Movie struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	DurationMinutes int       `json:"duration_minutes"`
	CreatedAt       time.Time `json:"created_at"`
}

type MovieDetail struct {
	ID               int64                 `json:"id"`
	Title            string                `json:"title"`
	DurationMinutes  int                   `json:"duration_minutes"`
	CreatedAt        time.Time             `json:"created_at"`
	Synopsis         *string               `json:"synopsis"`
	Director         *string               `json:"director"`
	CastMembers      *string               `json:"cast_members"`
	Genres           *string               `json:"genres"`
	PosterURL        *string               `json:"poster_url"`
	TrailerURL       *string               `json:"trailer_url"`
	ReleaseDate      *string               `json:"release_date"`
	AgeRating        *string               `json:"age_rating"`
	OriginalLanguage *string               `json:"original_language"`
	UpdatedAt        *time.Time            `json:"updated_at"`
	ExternalRatings  []MovieExternalRating `json:"external_ratings"`
}

type MovieExternalRating struct {
	ID           int64     `json:"id"`
	Source       string    `json:"source"`
	RatingScore  float64   `json:"rating_score"`
	MaxScore     float64   `json:"max_score"`
	VoteCount    *int64    `json:"vote_count"`
	ExternalURL  *string   `json:"external_url"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

type Showtime struct {
	ID       int64     `json:"id"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Movie    MovieInfo `json:"movie"`
	Cinema   PlaceInfo `json:"cinema"`
	Room     PlaceInfo `json:"room"`
}

type MovieInfo struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"duration_minutes"`
}

type PlaceInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type SeatStatus string

const (
	SeatAvailable SeatStatus = "AVAILABLE"
	SeatHeld      SeatStatus = "HELD"
	SeatBooked    SeatStatus = "BOOKED"
)

type Seat struct {
	ID     int64      `json:"id"`
	Number int        `json:"number"`
	Status SeatStatus `json:"status"`
}

type SeatRow struct {
	Label string `json:"label"`
	Seats []Seat `json:"seats"`
}

type SeatMap struct {
	ShowtimeID int64       `json:"showtime_id"`
	Movie      MovieInfo   `json:"movie"`
	Cinema     PlaceInfo   `json:"cinema"`
	Room       PlaceInfo   `json:"room"`
	StartsAt   time.Time   `json:"starts_at"`
	EndsAt     time.Time   `json:"ends_at"`
	Rows       []SeatRow   `json:"rows"`
	Summary    SeatSummary `json:"summary"`
}

type SeatSummary struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Held      int `json:"held"`
	Booked    int `json:"booked"`
}

// CatalogRepository is implemented by the infrastructure layer. The use case
// depends only on this contract, not on PostgreSQL or database/sql.
type CatalogRepository interface {
	ListCinemas(ctx context.Context) ([]Cinema, error)
	ListMoviesByCinema(ctx context.Context, cinemaID int64) ([]Movie, error)
	GetMovieDetail(ctx context.Context, movieID int64) (*MovieDetail, error)
	ListShowtimesByCinemaAndMovie(ctx context.Context, cinemaID, movieID int64) ([]Showtime, error)
	GetSeatMap(ctx context.Context, showtimeID int64) (*SeatMap, error)
}
