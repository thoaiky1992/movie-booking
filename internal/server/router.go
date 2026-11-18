package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	bookinghttp "movie-booking/modules/booking/presentation/http_api"
	cataloghttp "movie-booking/modules/catalog/presentation/http_api"
	identityhttp "movie-booking/modules/identity/presentation/http_api"
	systemhttp "movie-booking/modules/system/presentation/http_api"
	appconfig "movie-booking/shared/config"
	"movie-booking/shared/httpapi"
	"movie-booking/shared/httpapi/middlewares"
)

func newRouter(
	config appconfig.Config,
	logger *slog.Logger,
	authenticate gin.HandlerFunc,
	systemHandler *systemhttp.Handler,
	catalogHandler *cataloghttp.Handler,
	bookingHandler *bookinghttp.Handler,
) http.Handler {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(
		middlewares.TraceID(),
		middlewares.RequestLogger(logger),
		middlewares.Recovery(logger),
		cors.New(corsConfig(config.HTTP)),
	)

	router.NoRoute(func(c *gin.Context) {
		httpapi.RespondError(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	router.NoMethod(func(c *gin.Context) {
		httpapi.RespondError(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	router.GET("/", systemHandler.Root)
	router.GET("/health", systemHandler.Health)

	api := router.Group("/api/v1", authenticate)
	api.GET("/cinemas", catalogHandler.ListCinemas)
	api.GET("/cinemas/:cinemaID/movies", catalogHandler.ListMoviesByCinema)
	api.GET("/movies/:movieID", catalogHandler.GetMovieDetail)
	api.GET("/cinemas/:cinemaID/movies/:movieID/showtimes", catalogHandler.ListShowtimesByCinemaAndMovie)
	api.GET("/showtimes/:showtimeID/seats", catalogHandler.GetSeatMap)
	api.POST("/showtimes/:showtimeID/holds", bookingHandler.HoldSeats)
	api.POST("/bookings/:bookingID/confirm", bookingHandler.ConfirmBooking)
	api.POST("/bookings/:bookingID/cancel", bookingHandler.CancelBooking)
	return router
}

func corsConfig(config appconfig.HTTP) cors.Config {
	allowedOrigins := config.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = appconfig.DefaultHTTP().AllowedOrigins
	}
	return cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Accept", "Authorization", "Content-Type", httpapi.TraceIDHeader, identityhttp.UserIDHeader,
		},
		ExposeHeaders:    []string{httpapi.TraceIDHeader},
		AllowCredentials: true,
	}
}
