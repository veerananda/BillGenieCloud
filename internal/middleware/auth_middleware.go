package middleware

import (
	"log"
	"net/http"
	"strings"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT tokens
func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Println("❌ Missing Authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Println("❌ Invalid Authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			log.Printf("❌ Token validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Store user info in context
		c.Set("user_id", claims.UserID)
		c.Set("restaurant_id", claims.RestaurantID)
		c.Set("role", claims.Role)

		log.Printf("✅ User authenticated: %s (Role: %s)", claims.UserID, claims.Role)

		c.Next()
	}
}

// RoleMiddleware checks if user has required role
func RoleMiddleware(requiredRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "role information not found"})
			c.Abort()
			return
		}

		userRole := role.(string)
		allowed := false
		for _, r := range requiredRoles {
			if userRole == r {
				allowed = true
				break
			}
		}

		if !allowed {
			log.Printf("❌ Access denied for role: %s", userRole)
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ErrorHandling middleware for consistent error responses
func ErrorHandling() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			log.Printf("❌ Error: %v", err)

			statusCode := http.StatusInternalServerError
			if c.Writer.Status() != http.StatusOK {
				statusCode = c.Writer.Status()
			}

			c.JSON(statusCode, gin.H{
				"error": err.Error(),
			})
		}
	}
}

// CORSMiddleware handles CORS
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := false

		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// LoggingMiddleware logs all requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("📝 %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
		log.Printf("✅ %d %s %s", c.Writer.Status(), c.Request.Method, c.Request.URL.Path)
	}
}
