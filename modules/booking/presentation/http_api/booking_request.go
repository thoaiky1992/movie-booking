package http_api

type holdSeatsRequest struct {
	SeatIDs []int64 `json:"seat_ids" binding:"required,min=1,max=10,dive,gt=0"`
}
