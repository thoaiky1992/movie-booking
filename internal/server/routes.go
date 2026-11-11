package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(
		httpdelivery.TraceIDMiddleware(),
		httpdelivery.RequestLogger(s.logger),
		httpdelivery.Recovery(s.logger),
	)

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:5173"}, // Add your frontend URL
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			httpdelivery.TraceIDHeader,
			httpdelivery.UserIDHeader,
		},
		ExposeHeaders:    []string{httpdelivery.TraceIDHeader},
		AllowCredentials: true, // Enable cookies/auth
	}))
	r.NoRoute(func(c *gin.Context) {
		httpdelivery.RespondError(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	r.NoMethod(func(c *gin.Context) {
		httpdelivery.RespondError(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	r.GET("/", s.HelloWorldHandler)

	r.GET("/health", s.healthHandler)

	api := r.Group("/api/v1")
	{
		api.GET("/cinemas", s.catalogHandler.ListCinemas)
		api.GET("/cinemas/:cinemaID/movies", s.catalogHandler.ListMoviesByCinema)
		api.GET("/movies/:movieID", s.catalogHandler.GetMovieDetail)
		api.GET(
			"/cinemas/:cinemaID/movies/:movieID/showtimes",
			s.catalogHandler.ListShowtimesByCinemaAndMovie,
		)
		api.GET("/showtimes/:showtimeID/seats", s.catalogHandler.GetSeatMap)
		api.POST(
			"/showtimes/:showtimeID/holds",
			httpdelivery.AuthMiddleware(s.userRepository),
			s.bookingHandler.HoldSeats,
		)
		api.POST(
			"/bookings/:bookingID/cancel",
			httpdelivery.AuthMiddleware(s.userRepository),
			s.bookingHandler.CancelBooking,
		)
	}

	return r
}

func (s *Server) HelloWorldHandler(c *gin.Context) {
	httpdelivery.RespondSuccess(c, http.StatusOK, gin.H{"message": "Hello World"})
}

func (s *Server) healthHandler(c *gin.Context) {
	httpdelivery.RespondSuccess(c, http.StatusOK, s.db.Health())
}
