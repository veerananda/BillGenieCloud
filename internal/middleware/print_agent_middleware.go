package middleware

import (
	"net/http"
	"strings"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
)

// PrintAgentAuthMiddleware authenticates the on-site print agent via X-Print-Agent-Key or Bearer.
func PrintAgentAuthMiddleware(printService *services.PrintService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader("X-Print-Agent-Key"))
		if key == "" {
			auth := strings.TrimSpace(c.GetHeader("Authorization"))
			if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
				key = strings.TrimSpace(auth[7:])
			}
		}
		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing print agent key"})
			return
		}
		restaurantID, err := printService.FindRestaurantIDByAgentKey(key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid print agent key"})
			return
		}
		c.Set("restaurant_id", restaurantID)
		c.Set("print_agent", true)
		c.Next()
	}
}
