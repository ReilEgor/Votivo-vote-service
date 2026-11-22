package middleware

import (
	"crypto/sha256"
	"crypto/subtle"

	"github.com/gin-gonic/gin"
)

// HeaderInternalAuth carries the secret the gateway shares with the service.
const HeaderInternalAuth = "X-Internal-Auth"

// InternalAuth refuses every call that does not carry the shared secret. The gateway is the only party that verifies
// the user's token (and its DPoP proof) and the only one that may tell this service who the user is.
// An empty secret refuses everything instead of letting everything through.
func InternalAuth(secret string) gin.HandlerFunc {
	want := sha256.Sum256([]byte(secret))
	return func(c *gin.Context) {
		got := sha256.Sum256([]byte(c.GetHeader(HeaderInternalAuth)))
		// Hashing first makes the comparison constant-time for any input length.
		if secret == "" || subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			Fail(c, ErrUnauthorized)
			c.Abort()
			return
		}
		c.Next()
	}
}
