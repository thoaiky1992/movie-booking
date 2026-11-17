package server

import (
	"fmt"
	"log/slog"
	"net/http"

	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/domain"
)

type Dependencies struct {
	UserRepository domain.UserRepository
	SystemHandler  *httpdelivery.SystemHandler
	CatalogHandler *httpdelivery.CatalogHandler
	BookingHandler *httpdelivery.BookingHandler
	Logger         *slog.Logger
}

type Server struct {
	userRepository domain.UserRepository
	systemHandler  *httpdelivery.SystemHandler
	catalogHandler *httpdelivery.CatalogHandler
	bookingHandler *httpdelivery.BookingHandler
	logger         *slog.Logger
	allowedOrigins []string
}

func New(config Config, dependencies Dependencies) *http.Server {
	config = config.withDefaults()
	app := &Server{
		userRepository: dependencies.UserRepository,
		systemHandler:  dependencies.SystemHandler,
		catalogHandler: dependencies.CatalogHandler,
		bookingHandler: dependencies.BookingHandler,
		logger:         dependencies.Logger,
		allowedOrigins: append([]string(nil), config.AllowedOrigins...),
	}

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      app.RegisterRoutes(),
		IdleTimeout:  config.IdleTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}
}
