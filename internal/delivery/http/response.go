package http

import "github.com/gin-gonic/gin"

// SuccessResponse is the common envelope returned by successful endpoints.
type SuccessResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	TraceID string `json:"trace_id"`
}

// ErrorDetail contains a stable machine-readable code and a safe message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the common envelope returned by failed endpoints.
type ErrorResponse struct {
	Success bool        `json:"success"`
	Error   ErrorDetail `json:"error"`
	TraceID string      `json:"trace_id"`
}

func RespondSuccess(c *gin.Context, status int, data any) {
	c.JSON(status, SuccessResponse{
		Success: true,
		Data:    data,
		TraceID: EnsureTraceID(c),
	})
}

func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
		TraceID: EnsureTraceID(c),
	})
}
