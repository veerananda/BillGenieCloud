package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// PlatformAuthMiddleware protects BillGenie creator-only ops routes.
// Set PLATFORM_OPS_API_KEY in environment; clients send X-Platform-Api-Key header.
func PlatformAuthMiddleware() gin.HandlerFunc {
	expected := strings.TrimSpace(os.Getenv("PLATFORM_OPS_API_KEY"))
	return func(c *gin.Context) {
		if expected == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "platform ops API is not configured",
			})
			c.Abort()
			return
		}

		key := strings.TrimSpace(c.GetHeader("X-Platform-Api-Key"))
		if key == "" {
			auth := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				key = strings.TrimSpace(auth[7:])
			}
		}
		if key == "" || key != expected {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid platform credentials"})
			c.Abort()
			return
		}

		actor := strings.TrimSpace(c.GetHeader("X-Platform-Actor"))
		if actor == "" {
			actor = "platform_ops"
		}
		c.Set("platform_actor", actor)
		c.Next()
	}
}
