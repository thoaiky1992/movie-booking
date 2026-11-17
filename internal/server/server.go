package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"movie-booking/internal/config"
	httpdelivery "movie-booking/internal/delivery/http"
)

type Dependencies struct {
	Authenticate   gin.HandlerFunc
	SystemHandler  *httpdelivery.SystemHandler
	CatalogHandler *httpdelivery.CatalogHandler
	BookingHandler *httpdelivery.BookingHandler
	Logger         *slog.Logger
}

type Server struct {
	authenticate   gin.HandlerFunc
	systemHandler  *httpdelivery.SystemHandler
	catalogHandler *httpdelivery.CatalogHandler
	bookingHandler *httpdelivery.BookingHandler
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
