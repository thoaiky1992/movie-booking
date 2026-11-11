package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"

	"movie-booking/internal/database"
	httpdelivery "movie-booking/internal/delivery/http"
	"movie-booking/internal/domain"
)

type Server struct {
	port int

	db             database.Service
	userRepository domain.UserRepository
	catalogHandler *httpdelivery.CatalogHandler
	bookingHandler *httpdelivery.BookingHandler
	logger         *slog.Logger
}

func NewServer(
	db database.Service,
	userRepository domain.UserRepository,
	catalogHandler *httpdelivery.CatalogHandler,
	bookingHandler *httpdelivery.BookingHandler,
	logger *slog.Logger,
) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	if port == 0 {
		port = 8080
	}

	app := &Server{
		port:           port,
		db:             db,
		userRepository: userRepository,
		catalogHandler: catalogHandler,
		bookingHandler: bookingHandler,
		logger:         logger,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.port),
		Handler:      app.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
