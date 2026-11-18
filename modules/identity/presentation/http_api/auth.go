package http_api

import (
	nethttp "net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"movie-booking/modules/identity/application"
	httpdelivery "movie-booking/shared/httpapi"
)

const UserIDHeader = "X-User-ID"

// Auth resolves the caller identity at the HTTP boundary. In a production
// deployment, X-User-ID must be set by a trusted auth gateway.
func Auth(authenticator *application.AuthenticateUserQuery) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := strconv.ParseInt(strings.TrimSpace(c.GetHeader(UserIDHeader)), 10, 64)
		if err != nil || userID <= 0 {
			httpdelivery.RespondError(
				c,
				nethttp.StatusUnauthorized,
				"UNAUTHORIZED",
				"a valid X-User-ID header is required",
			)
			c.Abort()
			return
		}

		exists, err := authenticator.Authenticate(c.Request.Context(), userID)
		if err != nil {
			httpdelivery.RespondInternalError(c, err)
			c.Abort()
			return
		}
		if !exists {
			httpdelivery.RespondError(
				c,
				nethttp.StatusUnauthorized,
				"UNAUTHORIZED",
				"user identity is invalid",
			)
			c.Abort()
			return
		}

		c.Request = c.Request.WithContext(httpdelivery.ContextWithUserID(c.Request.Context(), userID))
		c.Next()
	}
}
