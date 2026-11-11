package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/domain"
)

type catalogUseCaseStub struct {
	requestedCinemaID      int64
	requestedMovieID       int64
	requestedMovieDetailID int64
	requestedShowtimeID    int64
}

type bookingUseCaseStub struct {
	userID     int64
	showtimeID int64
	seatIDs    []int64
}

type userRepositoryStub struct {
	exists bool
	err    error
}

func (s *userRepositoryStub) Exists(context.Context, int64) (bool, error) {
	return s.exists, s.err
}

func (s *bookingUseCaseStub) HoldSeats(
	_ context.Context,
	userID int64,
	showtimeID int64,
	seatIDs []int64,
) (*domain.SeatHold, error) {
	s.userID = userID
	s.showtimeID = showtimeID
	s.seatIDs = append([]int64(nil), seatIDs...)
	return &domain.SeatHold{
		BookingID:  userID,
		ShowtimeID: showtimeID,
		SeatIDs:    seatIDs,
		Status:     "PENDING",
	}, nil
}

func (s *catalogUseCaseStub) ListCinemas(context.Context) ([]domain.Cinema, error) {
	return []domain.Cinema{}, nil
}

func (s *catalogUseCaseStub) ListMoviesByCinema(
	_ context.Context,
	cinemaID int64,
) ([]domain.Movie, error) {
	s.requestedCinemaID = cinemaID
	return []domain.Movie{}, nil
}

func (s *catalogUseCaseStub) GetMovieDetail(
	_ context.Context,
	movieID int64,
) (*domain.MovieDetail, error) {
	s.requestedMovieDetailID = movieID
	return &domain.MovieDetail{
		ID: movieID,
		ExternalRatings: []domain.MovieExternalRating{
			{Source: "IMDb", RatingScore: 8.5, MaxScore: 10},
		},
	}, nil
}

func (s *catalogUseCaseStub) ListShowtimesByCinemaAndMovie(
	_ context.Context,
	cinemaID, movieID int64,
) ([]domain.Showtime, error) {
	s.requestedCinemaID = cinemaID
	s.requestedMovieID = movieID
	return []domain.Showtime{}, nil
}

func (s *catalogUseCaseStub) GetSeatMap(
	_ context.Context,
	showtimeID int64,
) (*domain.SeatMap, error) {
	s.requestedShowtimeID = showtimeID
	return &domain.SeatMap{ShowtimeID: showtimeID}, nil
}

func TestHelloWorldHandlerUsesSuccessEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := &Server{}
	r := gin.New()
	r.GET("/", s.HelloWorldHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(httpdelivery.TraceIDHeader, "trace-success-123")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get(httpdelivery.TraceIDHeader); got != "trace-success-123" {
		t.Fatalf("trace header = %q, want %q", got, "trace-success-123")
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Message string `json:"message"`
		} `json:"data"`
		TraceID string `json:"trace_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || response.Data.Message != "Hello World" || response.TraceID != "trace-success-123" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestRegisterRoutesUsesErrorEnvelopeAndJSONLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var logOutput bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logOutput, nil))
	s := &Server{logger: logger}
	r := s.RegisterRoutes()

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set(httpdelivery.TraceIDHeader, "trace-error-456")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	var response httpdelivery.ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Success || response.Error.Code != "ROUTE_NOT_FOUND" || response.TraceID != "trace-error-456" {
		t.Fatalf("unexpected response: %+v", response)
	}

	var entry map[string]any
	if err := json.Unmarshal(logOutput.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v\nlog: %s", err, logOutput.String())
	}
	if entry["msg"] != "http_request" || entry["trace_id"] != "trace-error-456" {
		t.Fatalf("unexpected log entry: %+v", entry)
	}
	if entry["status"] != float64(http.StatusNotFound) || entry["level"] != "WARN" {
		t.Fatalf("unexpected status or level in log entry: %+v", entry)
	}
}

func TestRegisterRoutesExposesNewCatalogAPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	useCase := &catalogUseCaseStub{}
	bookingUseCase := &bookingUseCaseStub{}
	s := &Server{
		userRepository: &userRepositoryStub{exists: true},
		catalogHandler: httpdelivery.NewCatalogHandler(useCase),
		bookingHandler: httpdelivery.NewBookingHandler(bookingUseCase),
		logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	r := s.RegisterRoutes()

	tests := []struct {
		path       string
		wantStatus int
	}{
		{path: "/api/v1/cinemas", wantStatus: http.StatusOK},
		{path: "/api/v1/cinemas/10/movies", wantStatus: http.StatusOK},
		{path: "/api/v1/movies/20", wantStatus: http.StatusOK},
		{path: "/api/v1/cinemas/10/movies/20/showtimes", wantStatus: http.StatusOK},
		{path: "/api/v1/showtimes/30/seats", wantStatus: http.StatusOK},
		{path: "/api/v1/movies", wantStatus: http.StatusNotFound},
		{path: "/api/v1/showtimes", wantStatus: http.StatusNotFound},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			r.ServeHTTP(recorder, req)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/showtimes/30/holds",
		bytes.NewBufferString(`{"seat_ids":[101,102]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(httpdelivery.UserIDHeader, "10")
	r.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("hold status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if bookingUseCase.showtimeID != 30 || bookingUseCase.userID != 10 {
		t.Fatalf(
			"unexpected hold arguments: user_id=%d showtime_id=%d seat_ids=%v",
			bookingUseCase.userID,
			bookingUseCase.showtimeID,
			bookingUseCase.seatIDs,
		)
	}

	unauthorizedRecorder := httptest.NewRecorder()
	unauthorizedRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/showtimes/30/holds",
		bytes.NewBufferString(`{"seat_ids":[101,102]}`),
	)
	unauthorizedRequest.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(unauthorizedRecorder, unauthorizedRequest)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"hold without X-User-ID status = %d, want %d; body = %s",
			unauthorizedRecorder.Code,
			http.StatusUnauthorized,
			unauthorizedRecorder.Body.String(),
		)
	}

	if useCase.requestedCinemaID != 10 {
		t.Fatalf("cinema ID = %d, want 10", useCase.requestedCinemaID)
	}
	if useCase.requestedMovieID != 20 {
		t.Fatalf("movie ID = %d, want 20", useCase.requestedMovieID)
	}
	if useCase.requestedMovieDetailID != 20 {
		t.Fatalf("movie detail ID = %d, want 20", useCase.requestedMovieDetailID)
	}
	if useCase.requestedShowtimeID != 30 {
		t.Fatalf("showtime ID = %d, want 30", useCase.requestedShowtimeID)
	}
}

func TestMovieDetailResponseIncludesExternalRatings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	useCase := &catalogUseCaseStub{}
	s := &Server{
		catalogHandler: httpdelivery.NewCatalogHandler(useCase),
		logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	r := s.RegisterRoutes()

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/movies/20", nil)
	r.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var response struct {
		Success bool               `json:"success"`
		Data    domain.MovieDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Success || len(response.Data.ExternalRatings) != 1 {
		t.Fatalf("unexpected movie detail response: %+v", response)
	}
	if response.Data.ExternalRatings[0].Source != "IMDb" {
		t.Fatalf("rating source = %q, want IMDb", response.Data.ExternalRatings[0].Source)
	}
}
