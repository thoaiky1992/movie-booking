package httpadapter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"movie_booking/internal/domain"
	"movie_booking/internal/usecase"
)

type handler struct{ d Deps }

func pathUUID(c *gin.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: %s must be a UUID", domain.ErrInvalidInput, name)
	}
	return id, nil
}

// bind writes a 400 response and returns false if the request is malformed.
func (h *handler) bindOrAbort(c *gin.Context, err error) bool {
	if err == nil {
		return true
	}
	h.respondError(c, fmt.Errorf("%w: %v", domain.ErrInvalidInput, err))
	return false
}

// ---- health ---------------------------------------------------------------

func (h *handler) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
	defer cancel()

	code, overall := http.StatusOK, "ok"
	checks := make(map[string]string, len(h.d.Readiness))
	for name, r := range h.d.Readiness {
		checks[name] = "up"
		if err := r.Ping(ctx); err != nil {
			checks[name] = "down"
			code, overall = http.StatusServiceUnavailable, "degraded"
		}
	}
	c.JSON(code, gin.H{"status": overall, "checks": checks})
}

// ---- users ----------------------------------------------------------------

type registerRequest struct {
	Email    string `json:"email" binding:"required"`
	FullName string `json:"full_name" binding:"required"`
}

func (h *handler) registerUser(c *gin.Context) {
	var req registerRequest
	if !h.bindOrAbort(c, c.ShouldBindJSON(&req)) {
		return
	}
	u, err := h.d.Users.Register(c.Request.Context(), req.Email, req.FullName)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": u.ID, "email": u.Email, "full_name": u.FullName, "created_at": u.CreatedAt})
}

// ---- catalog --------------------------------------------------------------

type movieDTO struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	DurationMinutes int       `json:"duration_minutes"`
	Genres          []string  `json:"genres"`
	Certificate     string    `json:"certificate"`
	ReleaseDate     *string   `json:"release_date,omitempty"`
	PosterURL       string    `json:"poster_url"`
}

func toMovieDTO(m domain.Movie) movieDTO {
	dto := movieDTO{
		ID: m.ID, Title: m.Title, Description: m.Description, DurationMinutes: m.DurationMinutes,
		Genres: m.Genres, Certificate: m.Certificate, PosterURL: m.PosterURL,
	}
	if dto.Genres == nil {
		dto.Genres = []string{}
	}
	if m.ReleaseDate != nil {
		s := m.ReleaseDate.Format("2006-01-02")
		dto.ReleaseDate = &s
	}
	return dto
}

func (h *handler) listMovies(c *gin.Context) {
	page, err := parsePage(c)
	if !h.bindOrAbort(c, err) {
		return
	}
	res, err := h.d.Catalog.ListMovies(c.Request.Context(), usecase.MovieFilter{
		Query: c.Query("q"), Genre: c.Query("genre"), Page: page,
	})
	if err != nil {
		h.respondError(c, err)
		return
	}
	data := make([]movieDTO, 0, len(res.Items))
	for _, m := range res.Items {
		data = append(data, toMovieDTO(m))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "pagination": pageMeta(res)})
}

// getMovie is the latency-critical endpoint (target p99 < 150ms). It performs
// one indexed row read and a bounded rating lookup. It never calls a partner.
func (h *handler) getMovie(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	d, err := h.d.Catalog.GetMovie(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	ratings := d.Ratings.Ratings
	if ratings == nil {
		ratings = []domain.RatingValue{}
	}
	c.JSON(http.StatusOK, gin.H{
		"movie": toMovieDTO(d.Movie),
		"ratings": gin.H{
			"status":     d.Ratings.Status,
			"sources":    ratings,
			"updated_at": d.Ratings.UpdatedAt,
		},
	})
}

type cinemaDTO struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Address string    `json:"address"`
	City    string    `json:"city"`
}

type hallDTO struct {
	ID       uuid.UUID `json:"id"`
	CinemaID uuid.UUID `json:"cinema_id"`
	Name     string    `json:"name"`
}

func (h *handler) listCinemas(c *gin.Context) {
	page, err := parsePage(c)
	if !h.bindOrAbort(c, err) {
		return
	}
	res, err := h.d.Catalog.ListCinemas(c.Request.Context(), page)
	if err != nil {
		h.respondError(c, err)
		return
	}
	data := make([]cinemaDTO, 0, len(res.Items))
	for _, cin := range res.Items {
		data = append(data, cinemaDTO{ID: cin.ID, Name: cin.Name, Address: cin.Address, City: cin.City})
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "pagination": pageMeta(res)})
}

func (h *handler) listHalls(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	halls, err := h.d.Catalog.ListHalls(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	data := make([]hallDTO, 0, len(halls))
	for _, hl := range halls {
		data = append(data, hallDTO{ID: hl.ID, CinemaID: hl.CinemaID, Name: hl.Name})
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *handler) listShowtimes(c *gin.Context) {
	page, err := parsePage(c)
	if !h.bindOrAbort(c, err) {
		return
	}
	f := usecase.ShowtimeFilter{Page: page}
	if f.MovieID, err = optionalUUID(c, "movie_id"); !h.bindOrAbort(c, err) {
		return
	}
	if f.CinemaID, err = optionalUUID(c, "cinema_id"); !h.bindOrAbort(c, err) {
		return
	}
	if f.From, err = optionalTime(c, "from"); !h.bindOrAbort(c, err) {
		return
	}
	if f.To, err = optionalTime(c, "to"); !h.bindOrAbort(c, err) {
		return
	}

	res, err := h.d.Catalog.ListShowtimes(c.Request.Context(), f)
	if err != nil {
		h.respondError(c, err)
		return
	}
	data := make([]showtimeDTO, 0, len(res.Items))
	for _, v := range res.Items {
		data = append(data, showtimeDTO{
			ID: v.ID, MovieID: v.MovieID, MovieTitle: v.MovieTitle,
			CinemaID: v.CinemaID, CinemaName: v.CinemaName, HallID: v.HallID, HallName: v.HallName,
			StartsAt: v.StartsAt, EndsAt: v.EndsAt, PriceCents: v.BasePriceCents,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "pagination": pageMeta(res)})
}

type showtimeDTO struct {
	ID         uuid.UUID `json:"id"`
	MovieID    uuid.UUID `json:"movie_id"`
	MovieTitle string    `json:"movie_title"`
	CinemaID   uuid.UUID `json:"cinema_id"`
	CinemaName string    `json:"cinema_name"`
	HallID     uuid.UUID `json:"hall_id"`
	HallName   string    `json:"hall_name"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	PriceCents int64     `json:"base_price_cents"`
}

type seatDTO struct {
	ID         uuid.UUID `json:"id"`
	RowLabel   string    `json:"row_label"`
	SeatNumber int       `json:"seat_number"`
	SeatType   string    `json:"seat_type"`
	Status     string    `json:"status"`
	PriceCents int64     `json:"price_cents"`
}

func (h *handler) seatMap(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	m, err := h.d.Catalog.SeatMap(c.Request.Context(), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	seats := make([]seatDTO, 0, len(m.Seats))
	for _, s := range m.Seats {
		seats = append(seats, seatDTO{
			ID: s.ID, RowLabel: s.RowLabel, SeatNumber: s.SeatNumber, SeatType: s.SeatType,
			Status: s.Status, PriceCents: s.PriceCents,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"showtime": gin.H{"id": m.Showtime.ID, "starts_at": m.Showtime.StartsAt, "ends_at": m.Showtime.EndsAt},
		"seats":    seats,
	})
}

// ---- booking --------------------------------------------------------------

type holdRequest struct {
	SeatIDs []uuid.UUID `json:"seat_ids" binding:"required,min=1,max=10"`
}

type seatLineDTO struct {
	SeatID     uuid.UUID `json:"seat_id"`
	RowLabel   string    `json:"row_label"`
	SeatNumber int       `json:"seat_number"`
	SeatType   string    `json:"seat_type"`
	PriceCents int64     `json:"price_cents"`
}

type paymentDTO struct {
	ID          uuid.UUID `json:"id"`
	Status      string    `json:"status"`
	AmountCents int64     `json:"amount_cents"`
	RefundCents int64     `json:"refund_cents"`
}

type bookingDTO struct {
	ID          uuid.UUID     `json:"id"`
	Status      string        `json:"status"`
	ShowtimeID  uuid.UUID     `json:"showtime_id"`
	MovieTitle  string        `json:"movie_title,omitempty"`
	CinemaName  string        `json:"cinema_name,omitempty"`
	HallName    string        `json:"hall_name,omitempty"`
	StartsAt    *time.Time    `json:"starts_at,omitempty"`
	Seats       []seatLineDTO `json:"seats"`
	TotalCents  int64         `json:"total_cents"`
	Currency    string        `json:"currency"`
	TicketCode  string        `json:"ticket_code,omitempty"`
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"`
	ConfirmedAt *time.Time    `json:"confirmed_at,omitempty"`
	CancelledAt *time.Time    `json:"cancelled_at,omitempty"`
	Payment     *paymentDTO   `json:"payment,omitempty"`
}

func (h *handler) bookingFromDomain(b domain.Booking) bookingDTO {
	dto := bookingDTO{
		ID: b.ID, Status: string(b.Status), ShowtimeID: b.ShowtimeID, Seats: seatLines(b.Seats),
		TotalCents: b.TotalCents, Currency: h.d.Currency, TicketCode: b.TicketCode,
		ExpiresAt: b.ExpiresAt, ConfirmedAt: b.ConfirmedAt, CancelledAt: b.CancelledAt,
	}
	return dto
}

func (h *handler) bookingFromDetail(d *usecase.BookingDetail) bookingDTO {
	dto := h.bookingFromDomain(d.Booking)
	starts := d.StartsAt
	dto.MovieTitle, dto.CinemaName, dto.HallName, dto.StartsAt = d.MovieTitle, d.CinemaName, d.HallName, &starts
	if d.Payment != nil {
		dto.Payment = &paymentDTO{
			ID: d.Payment.ID, Status: string(d.Payment.Status),
			AmountCents: d.Payment.AmountCents, RefundCents: d.Payment.RefundCents,
		}
	}
	return dto
}

func seatLines(in []domain.SeatLine) []seatLineDTO {
	out := make([]seatLineDTO, 0, len(in))
	for _, s := range in {
		out = append(out, seatLineDTO{
			SeatID: s.SeatID, RowLabel: s.RowLabel, SeatNumber: s.SeatNumber,
			SeatType: s.SeatType, PriceCents: s.PriceCents,
		})
	}
	return out
}

func (h *handler) hold(c *gin.Context) {
	showtimeID, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	var req holdRequest
	if !h.bindOrAbort(c, c.ShouldBindJSON(&req)) {
		return
	}
	b, err := h.d.Booking.Hold(c.Request.Context(), currentUser(c), showtimeID, req.SeatIDs)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.bookingFromDomain(*b))
}

func (h *handler) getBooking(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	d, err := h.d.Booking.Get(c.Request.Context(), currentUser(c), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.bookingFromDetail(d))
}

func (h *handler) pay(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	d, err := h.d.Booking.Pay(c.Request.Context(), currentUser(c), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, h.bookingFromDetail(d))
}

func (h *handler) cancel(c *gin.Context) {
	id, err := pathUUID(c, "id")
	if !h.bindOrAbort(c, err) {
		return
	}
	res, err := h.d.Booking.Cancel(c.Request.Context(), currentUser(c), id)
	if err != nil {
		h.respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"booking_id":    res.BookingID,
		"status":        res.Status,
		"refund_cents":  res.RefundCents,
		"refund_status": res.RefundStatus,
	})
}

// ---- query helpers --------------------------------------------------------

func optionalUUID(c *gin.Context, key string) (*uuid.UUID, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be a UUID", domain.ErrInvalidInput, key)
	}
	return &id, nil
}

func optionalTime(c *gin.Context, key string) (*time.Time, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be RFC3339", domain.ErrInvalidInput, key)
	}
	return &t, nil
}
