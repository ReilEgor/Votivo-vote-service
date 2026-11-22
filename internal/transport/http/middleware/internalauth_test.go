package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ReilEgor/Votivo-vote-service/internal/transport/http/middleware"
)

func engine(secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.HandleErrors())
	api := r.Group("/api/v1", middleware.InternalAuth(secret), middleware.UserID())
	api.GET("/whoami", func(c *gin.Context) { c.String(http.StatusOK, middleware.UserIDFrom(c)) })
	return r
}

func call(r *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/whoami", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestInternalAuth(t *testing.T) {
	const secret = "gateway-secret"
	hdr := func(secret, user string) map[string]string {
		h := map[string]string{}
		if secret != "" {
			h["X-Internal-Auth"] = secret
		}
		if user != "" {
			h["X-User-ID"] = user
		}
		return h
	}
	tests := []struct {
		name    string
		secret  string
		headers map[string]string
		status  int
		body    string
	}{
		{"gateway with the secret", secret, hdr(secret, "u1"), http.StatusOK, "u1"},
		{"client naming a user without the secret", secret, hdr("", "victim"), http.StatusUnauthorized, ""},
		{"wrong secret", secret, hdr("guess", "u1"), http.StatusUnauthorized, ""},
		{"longer secret with the right prefix", secret, hdr(secret+"x", "u1"), http.StatusUnauthorized, ""},
		{"right secret but no user", secret, hdr(secret, ""), http.StatusUnauthorized, ""},
		{"unconfigured secret refuses everything", "", hdr("", "u1"), http.StatusUnauthorized, ""},
		{"unconfigured secret and a guessed header", "", hdr("x", "u1"), http.StatusUnauthorized, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := call(engine(tc.secret), tc.headers)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.status, w.Body.String())
			}
			if tc.status == http.StatusOK && w.Body.String() != tc.body {
				t.Fatalf("user = %q, want %q", w.Body.String(), tc.body)
			}
		})
	}
}
