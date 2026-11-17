package http

import (
	nethttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ParsePositiveID(c *gin.Context, parameter, code, resource string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(parameter), 10, 64)
	if err != nil || id <= 0 {
		RespondError(
			c,
			nethttp.StatusBadRequest,
			code,
			resource+" id must be a positive integer",
		)
		return 0, false
	}
	return id, true
}
