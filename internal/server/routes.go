package server

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) registerSystemRoutes(router *gin.Engine) {
	router.GET("/", s.systemHandler.Root)
	router.GET("/health", s.systemHandler.Health)
}

func (s *Server) registerAPIRoutes(api *gin.RouterGroup) {
	api.Use(s.authenticate)

	api.GET("/cinemas", s.catalogHandler.ListCinemas)
	api.GET("/cinemas/:cinemaID/movies", s.catalogHandler.ListMoviesByCinema)
	api.GET("/movies/:movieID", s.catalogHandler.GetMovieDetail)
	api.GET(
		"/cinemas/:cinemaID/movies/:movieID/showtimes",
		s.catalogHandler.ListShowtimesByCinemaAndMovie,
	)
	api.GET("/showtimes/:showtimeID/seats", s.catalogHandler.GetSeatMap)

	api.POST("/showtimes/:showtimeID/holds", s.bookingHandler.HoldSeats)
	api.POST("/bookings/:bookingID/confirm", s.bookingHandler.ConfirmBooking)
	api.POST("/bookings/:bookingID/cancel", s.bookingHandler.CancelBooking)
}
