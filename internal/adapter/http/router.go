// Package httpadapter exposes the use cases over HTTP (Gin).
package httpadapter

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

type Deps struct {
	Catalog     *usecase.CatalogService
	Booking     *usecase.BookingService
	Users       *usecase.UserService
	Readiness   map[string]usecase.Readiness
	CORSOrigins []string
	Currency    string
	Log         *slog.Logger
}

const ctxUserID = "userID"

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), accessLog(d.Log))
	if len(d.CORSOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     d.CORSOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Accept", "Content-Type", "X-User-ID"},
			AllowCredentials: false,
		}))
	}

	h := &handler{d: d}
	r.GET("/health", h.health)

	api := r.Group("/api/v1")
	api.POST("/users", h.registerUser)

	// Public catalog: read-only, cacheable, no per-user state.
	api.GET("/movies", h.listMovies)
	api.GET("/movies/:id", h.getMovie)
	api.GET("/cinemas", h.listCinemas)
	api.GET("/cinemas/:id/halls", h.listHalls)
	api.GET("/showtimes", h.listShowtimes)
	api.GET("/showtimes/:id/seats", h.seatMap)

	// Customer flows.
	auth := api.Group("", requireUser)
	auth.POST("/showtimes/:id/holds", h.hold)
	auth.GET("/bookings/:id", h.getBooking)
	auth.POST("/bookings/:id/payments", h.pay)
	auth.POST("/bookings/:id/cancel", h.cancel)

	return r
}

// requireUser reads the caller identity from X-User-ID.
// PLACEHOLDER: production must verify a signed token (JWT) at the gateway and
// forward only the verified subject. Do not expose this header to the internet as-is.
func requireUser(c *gin.Context) {
	id, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		abortWithError(c, http.StatusUnauthorized, "unauthorized", "missing or invalid X-User-ID header")
		return
	}
	c.Set(ctxUserID, id)
	c.Next()
}

func currentUser(c *gin.Context) uuid.UUID {
	id, _ := c.Get(ctxUserID)
	return id.(uuid.UUID)
}

func accessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000,
		)
	}
}

// ---- error mapping --------------------------------------------------------

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func abortWithError(c *gin.Context, status int, code, msg string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = msg
	c.AbortWithStatusJSON(status, body)
}

// respondError maps domain errors to HTTP semantics. Internal details are logged, never returned.
func (h *handler) respondError(c *gin.Context, err error) {
	status, code := classify(err)
	if status == http.StatusInternalServerError {
		h.d.Log.Error("request failed", "path", c.FullPath(), "err", err)
		abortWithError(c, status, code, "internal error")
		return
	}
	abortWithError(c, status, code, err.Error())
}

func classify(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "invalid_input"
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrSeatUnavailable):
		return http.StatusConflict, "seat_unavailable"
	case errors.Is(err, domain.ErrPaymentInProgress):
		return http.StatusConflict, "payment_in_progress"
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, "conflict"
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrShowtimeStarted):
		return http.StatusConflict, "invalid_state"
	case errors.Is(err, domain.ErrHoldExpired), errors.Is(err, domain.ErrNotConfirmable):
		return http.StatusGone, "hold_expired"
	case errors.Is(err, domain.ErrPaymentDeclined):
		return http.StatusPaymentRequired, "payment_declined"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

// ---- pagination helpers ---------------------------------------------------

func parsePage(c *gin.Context) (usecase.Page, error) {
	number, err := intQuery(c, "page", 1)
	if err != nil {
		return usecase.Page{}, err
	}
	size, err := intQuery(c, "page_size", usecase.DefaultPageSize)
	if err != nil {
		return usecase.Page{}, err
	}
	return usecase.NewPage(number, size), nil
}

func intQuery(c *gin.Context, key string, def int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, domain.ErrInvalidInput
	}
	return n, nil
}

type pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func pageMeta[T any](p usecase.Paged[T]) pagination {
	pages := 0
	if p.Total > 0 {
		pages = (p.Total + p.Page.Size - 1) / p.Page.Size
	}
	return pagination{Page: p.Page.Number, PageSize: p.Page.Size, Total: p.Total, TotalPages: pages}
}
