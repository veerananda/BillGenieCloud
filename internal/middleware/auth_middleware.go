package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

		// Validate user session (enforce single concurrent login for staff/chef)
		// Note: Skip session validation for admin users (they can have multiple sessions)
		if claims.Role != "admin" {
			isValid, err := authService.ValidateUserSession(claims.UserID, token)
			if err != nil {
				log.Printf("⚠️  Session validation failed: %v", err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				c.Abort()
				return
			}
			if !isValid {
				log.Printf("❌ User %s attempted to use invalidated session", claims.UserID)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "session invalidated. Another device has logged in with your account"})
				c.Abort()
				return
			}
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

// SubscriptionMiddleware checks if restaurant subscription is active
// Allows 30-day free trial, then requires active subscription
func SubscriptionMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip subscription check for public endpoints and auth endpoints
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/public/") || path == "/health" {
			c.Next()
			return
		}

		restaurantID, exists := c.Get("restaurant_id")
		if !exists {
			// If no restaurant_id, let auth middleware handle it
			c.Next()
			return
		}

		// Check restaurant subscription status
		var restaurant models.Restaurant
		if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
			log.Printf("❌ Failed to fetch restaurant: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check subscription status"})
			c.Abort()
			return
		}

		// Check if subscription has expired
		if time.Now().After(restaurant.SubscriptionEnd) {
			daysExpired := int(time.Since(restaurant.SubscriptionEnd).Hours() / 24)
			log.Printf("⚠️ Subscription expired for restaurant %s (%d days ago)", restaurantID, daysExpired)

			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":            "subscription_expired",
				"message":          "Your 30-day free trial has ended. Please subscribe to continue using BillGenie.",
				"subscription_end": restaurant.SubscriptionEnd,
				"days_expired":     daysExpired,
			})
			c.Abort()
			return
		}

		// Calculate days remaining in trial/subscription
		daysRemaining := int(time.Until(restaurant.SubscriptionEnd).Hours() / 24)

		// Add subscription info to context for use in handlers
		c.Set("subscription_end", restaurant.SubscriptionEnd)
		c.Set("days_remaining", daysRemaining)

		// Log warning if subscription is expiring soon (less than 7 days)
		if daysRemaining <= 7 && daysRemaining > 0 {
			log.Printf("⚠️ Subscription expiring soon for restaurant %s (%d days remaining)", restaurantID, daysRemaining)
		}

		c.Next()
	}
}
