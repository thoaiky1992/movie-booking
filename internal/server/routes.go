package server

import (
	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
)

func (s *Server) registerSystemRoutes(router *gin.Engine) {
	router.GET("/", s.systemHandler.Root)
	router.GET("/health", s.systemHandler.Health)
}

func (s *Server) registerAPIRoutes(api *gin.RouterGroup) {
	api.GET("/cinemas", s.catalogHandler.ListCinemas)
	api.GET("/cinemas/:cinemaID/movies", s.catalogHandler.ListMoviesByCinema)
	api.GET("/movies/:movieID", s.catalogHandler.GetMovieDetail)
	api.GET(
		"/cinemas/:cinemaID/movies/:movieID/showtimes",
		s.catalogHandler.ListShowtimesByCinemaAndMovie,
	)
	api.GET("/showtimes/:showtimeID/seats", s.catalogHandler.GetSeatMap)

	authenticate := httpdelivery.AuthMiddleware(s.userRepository)
	api.POST("/showtimes/:showtimeID/holds", authenticate, s.bookingHandler.HoldSeats)
	api.POST("/bookings/:bookingID/confirm", authenticate, s.bookingHandler.ConfirmBooking)
	api.POST("/bookings/:bookingID/cancel", authenticate, s.bookingHandler.CancelBooking)
}
