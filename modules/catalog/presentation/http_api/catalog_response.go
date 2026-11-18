package http_api

import (
	"time"

	"movie-booking/modules/catalog/domain"
)

type cinemaResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type movieResponse struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	DurationMinutes int       `json:"duration_minutes"`
	CreatedAt       time.Time `json:"created_at"`
}

type movieDetailResponse struct {
	ID               int64                         `json:"id"`
	Title            string                        `json:"title"`
	DurationMinutes  int                           `json:"duration_minutes"`
	CreatedAt        time.Time                     `json:"created_at"`
	Synopsis         *string                       `json:"synopsis"`
	Director         *string                       `json:"director"`
	CastMembers      *string                       `json:"cast_members"`
	Genres           *string                       `json:"genres"`
	PosterURL        *string                       `json:"poster_url"`
	TrailerURL       *string                       `json:"trailer_url"`
	ReleaseDate      *string                       `json:"release_date"`
	AgeRating        *string                       `json:"age_rating"`
	OriginalLanguage *string                       `json:"original_language"`
	UpdatedAt        *time.Time                    `json:"updated_at"`
	ExternalRatings  []movieExternalRatingResponse `json:"external_ratings"`
}

type movieExternalRatingResponse struct {
	ID           int64     `json:"id"`
	Source       string    `json:"source"`
	RatingScore  float64   `json:"rating_score"`
	MaxScore     float64   `json:"max_score"`
	VoteCount    *int64    `json:"vote_count"`
	ExternalURL  *string   `json:"external_url"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

type showtimeResponse struct {
	ID       int64             `json:"id"`
	StartsAt time.Time         `json:"starts_at"`
	EndsAt   time.Time         `json:"ends_at"`
	Movie    movieInfoResponse `json:"movie"`
	Cinema   placeInfoResponse `json:"cinema"`
	Room     placeInfoResponse `json:"room"`
}

type movieInfoResponse struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	DurationMinutes int    `json:"duration_minutes"`
}

type placeInfoResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type seatResponse struct {
	ID     int64             `json:"id"`
	Number int               `json:"number"`
	Status domain.SeatStatus `json:"status"`
}

type seatRowResponse struct {
	Label string         `json:"label"`
	Seats []seatResponse `json:"seats"`
}

type seatMapResponse struct {
	ShowtimeID int64               `json:"showtime_id"`
	Movie      movieInfoResponse   `json:"movie"`
	Cinema     placeInfoResponse   `json:"cinema"`
	Room       placeInfoResponse   `json:"room"`
	StartsAt   time.Time           `json:"starts_at"`
	EndsAt     time.Time           `json:"ends_at"`
	Rows       []seatRowResponse   `json:"rows"`
	Summary    seatSummaryResponse `json:"summary"`
}

type seatSummaryResponse struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Held      int `json:"held"`
	Booked    int `json:"booked"`
}

func newCinemaResponses(cinemas []domain.Cinema) []cinemaResponse {
	responses := make([]cinemaResponse, len(cinemas))
	for i, cinema := range cinemas {
		responses[i] = cinemaResponse{
			ID: cinema.ID, Name: cinema.Name, CreatedAt: cinema.CreatedAt,
		}
	}
	return responses
}

func newMovieResponses(movies []domain.Movie) []movieResponse {
	responses := make([]movieResponse, len(movies))
	for i, movie := range movies {
		responses[i] = movieResponse{
			ID: movie.ID, Title: movie.Title,
			DurationMinutes: movie.DurationMinutes, CreatedAt: movie.CreatedAt,
		}
	}
	return responses
}

func newMovieDetailResponse(movie *domain.MovieDetail) *movieDetailResponse {
	if movie == nil {
		return nil
	}
	ratings := make([]movieExternalRatingResponse, len(movie.ExternalRatings))
	for i, rating := range movie.ExternalRatings {
		ratings[i] = movieExternalRatingResponse{
			ID: rating.ID, Source: rating.Source,
			RatingScore: rating.RatingScore, MaxScore: rating.MaxScore,
			VoteCount: rating.VoteCount, ExternalURL: rating.ExternalURL,
			LastSyncedAt: rating.LastSyncedAt,
		}
	}
	return &movieDetailResponse{
		ID: movie.ID, Title: movie.Title,
		DurationMinutes: movie.DurationMinutes, CreatedAt: movie.CreatedAt,
		Synopsis: movie.Synopsis, Director: movie.Director,
		CastMembers: movie.CastMembers, Genres: movie.Genres,
		PosterURL: movie.PosterURL, TrailerURL: movie.TrailerURL,
		ReleaseDate: movie.ReleaseDate, AgeRating: movie.AgeRating,
		OriginalLanguage: movie.OriginalLanguage, UpdatedAt: movie.UpdatedAt,
		ExternalRatings: ratings,
	}
}

func newShowtimeResponses(showtimes []domain.Showtime) []showtimeResponse {
	responses := make([]showtimeResponse, len(showtimes))
	for i, showtime := range showtimes {
		responses[i] = showtimeResponse{
			ID: showtime.ID, StartsAt: showtime.StartsAt, EndsAt: showtime.EndsAt,
			Movie:  newMovieInfoResponse(showtime.Movie),
			Cinema: newPlaceInfoResponse(showtime.Cinema),
			Room:   newPlaceInfoResponse(showtime.Room),
		}
	}
	return responses
}

func newSeatMapResponse(seatMap *domain.SeatMap) *seatMapResponse {
	if seatMap == nil {
		return nil
	}
	rows := make([]seatRowResponse, len(seatMap.Rows))
	for i, row := range seatMap.Rows {
		seats := make([]seatResponse, len(row.Seats))
		for j, seat := range row.Seats {
			seats[j] = seatResponse{ID: seat.ID, Number: seat.Number, Status: seat.Status}
		}
		rows[i] = seatRowResponse{Label: row.Label, Seats: seats}
	}
	return &seatMapResponse{
		ShowtimeID: seatMap.ShowtimeID,
		Movie:      newMovieInfoResponse(seatMap.Movie),
		Cinema:     newPlaceInfoResponse(seatMap.Cinema),
		Room:       newPlaceInfoResponse(seatMap.Room),
		StartsAt:   seatMap.StartsAt,
		EndsAt:     seatMap.EndsAt,
		Rows:       rows,
		Summary: seatSummaryResponse{
			Total: seatMap.Summary.Total, Available: seatMap.Summary.Available,
			Held: seatMap.Summary.Held, Booked: seatMap.Summary.Booked,
		},
	}
}

func newMovieInfoResponse(movie domain.MovieInfo) movieInfoResponse {
	return movieInfoResponse{
		ID: movie.ID, Title: movie.Title, DurationMinutes: movie.DurationMinutes,
	}
}

func newPlaceInfoResponse(place domain.PlaceInfo) placeInfoResponse {
	return placeInfoResponse{ID: place.ID, Name: place.Name}
}
