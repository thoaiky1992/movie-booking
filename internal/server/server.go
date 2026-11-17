package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/internal/config"
	bookingdelivery "movie-booking/internal/delivery/http/booking"
	catalogdelivery "movie-booking/internal/delivery/http/catalog"
	systemdelivery "movie-booking/internal/delivery/http/system"
)

type Dependencies struct {
	Authenticate   gin.HandlerFunc
	SystemHandler  *systemdelivery.Handler
	CatalogHandler *catalogdelivery.Handler
	BookingHandler *bookingdelivery.Handler
	Logger         *slog.Logger
}

type Server struct {
	authenticate   gin.HandlerFunc
	systemHandler  *systemdelivery.Handler
	catalogHandler *catalogdelivery.Handler
	bookingHandler *bookingdelivery.Handler
	logger         *slog.Logger
	allowedOrigins []string
}

func New(httpConfig config.HTTP, dependencies Dependencies) *http.Server {
	app := &Server{
		authenticate:   dependencies.Authenticate,
		systemHandler:  dependencies.SystemHandler,
		catalogHandler: dependencies.CatalogHandler,
		bookingHandler: dependencies.BookingHandler,
		logger:         dependencies.Logger,
		allowedOrigins: append([]string(nil), httpConfig.AllowedOrigins...),
	}

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", httpConfig.Port),
		Handler:      app.RegisterRoutes(),
		IdleTimeout:  httpConfig.IdleTimeout,
		ReadTimeout:  httpConfig.ReadTimeout,
		WriteTimeout: httpConfig.WriteTimeout,
	}
}
