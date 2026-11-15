package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ReilEgor/Votivo-vote-service/internal/transport/http/dto"
	"github.com/ReilEgor/Votivo-vote-service/pkg/ctxlog"
)

const (
	// HeaderRequestID carries the ID of the business process.
	HeaderRequestID = "X-Request-ID"
	// HeaderUserID carries the ID of the user; the API gateway sets it after it has verified the JWT.
	HeaderUserID = "X-User-ID"

	maxRequestIDLen = 128
	maxUserIDLen    = 128
)

// RequestID puts the request ID into the request context and the response header.
// The client's X-Request-ID is used when present and sane, otherwise a new UUID is created.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" || len(id) > maxRequestIDLen {
			id = uuid.NewString()
		}
		c.Header(HeaderRequestID, id)
		c.Request = c.Request.WithContext(ctxlog.WithRequestID(c.Request.Context(), id))
		c.Next()
	}
}

// Recovery turns a panic into a 500 response and logs it with the stack.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(c.Request.Context(), "panic recovered",
					slog.String("panic", fmt.Sprint(r)),
					slog.String("stack", string(debug.Stack())),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "internal error"})
			}
		}()
		c.Next()
	}
}

// UserID takes the user from X-User-ID and makes it the initiator of everything logged for the request.
// The service is reachable only through the gateway, which strips the header from outside requests and sets it
// from the verified token, so the value is trusted here.
func UserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderUserID)
		if id == "" || len(id) > maxUserIDLen {
			Fail(c, ErrUnauthorized)
			c.Abort()
			return
		}
		c.Set(userIDKey, id)
		c.Request = c.Request.WithContext(ctxlog.WithInitiator(c.Request.Context(), id))
		c.Next()
	}
}

const userIDKey = "user_id"

// UserIDFrom returns the user stored by UserID.
func UserIDFrom(c *gin.Context) string {
	return c.GetString(userIDKey)
}
