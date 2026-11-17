package server

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	httpdelivery "movie-booking/internal/delivery/http"
)

func (s *Server) RegisterRoutes() http.Handler {
	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(
		httpdelivery.TraceIDMiddleware(),
		httpdelivery.RequestLogger(s.logger),
		httpdelivery.Recovery(s.logger),
		cors.New(s.corsConfig()),
	)

	router.NoRoute(func(c *gin.Context) {
		httpdelivery.RespondError(c, http.StatusNotFound, "ROUTE_NOT_FOUND", "route not found")
	})
	router.NoMethod(func(c *gin.Context) {
		httpdelivery.RespondError(c, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})

	s.registerSystemRoutes(router)
	s.registerAPIRoutes(router.Group("/api/v1"))
	return router
}

func (s *Server) corsConfig() cors.Config {
	allowedOrigins := s.allowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = DefaultConfig().AllowedOrigins
	}

	return cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			httpdelivery.TraceIDHeader,
			httpdelivery.UserIDHeader,
		},
		ExposeHeaders:    []string{httpdelivery.TraceIDHeader},
		AllowCredentials: true,
	}
}
