package http

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ReilEgor/Votivo-vote-service/internal/transport/http/middleware"
)

// NewRouter returns the Gin engine with all routes. probe serves /health and /version.
// The service sits behind the gateway, so no proxy is trusted and the TCP peer is always the caller. Every /api call
// must carry internalSecret in X-Internal-Auth: only the gateway verifies the user's token, so a caller that reaches
// the service directly must not be able to name the user.
func NewRouter(h *Handler, probe http.Handler, internalSecret string) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	r.Use(middleware.RequestID(), middleware.AccessLog(), middleware.Recovery(), middleware.HandleErrors())

	r.GET("/health", gin.WrapH(probe))
	r.GET("/version", gin.WrapH(probe))

	api := r.Group("/api/v1", middleware.InternalAuth(internalSecret), middleware.UserID())
	api.POST("/polls/:poll_id/vote", h.Cast)
	api.PUT("/polls/:poll_id/vote", h.Change)
	api.DELETE("/polls/:poll_id/vote", h.Revoke)
	api.GET("/polls/:poll_id/vote", h.Status)
	return r, nil
}
