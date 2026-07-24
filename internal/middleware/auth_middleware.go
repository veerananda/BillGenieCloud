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

		// Enforce single active session per user (all roles).
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

// CORSMiddleware handles CORS against an explicit origin allowlist.
// Wildcard (*) is only tolerated outside production (validated at config load);
// when used, credentials are never enabled.
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	allowAll := false
	exact := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		if o == "*" {
			allowAll = true
			continue
		}
		exact[o] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if allowAll {
			// Non-production wildcard: echo * without credentials.
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Platform-Api-Key, X-Platform-Actor")
		} else if origin != "" {
			if _, ok := exact[origin]; ok {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Vary", "Origin")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Platform-Api-Key, X-Platform-Actor")
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// OriginAllowed reports whether origin is on the CORS allowlist (used by WebSocket CheckOrigin).
func OriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return true // non-browser clients (native apps) often omit Origin
	}
	for _, o := range allowedOrigins {
		if o == "*" || o == origin {
			return true
		}
	}
	return false
}

// LoggingMiddleware logs all requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("📝 %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
		log.Printf("✅ %d %s %s", c.Writer.Status(), c.Request.Method, c.Request.URL.Path)
	}
}

// SubscriptionMiddleware checks if restaurant subscription is active.
// Skips auth/public routes, subscription payment endpoints, and profile read for paywall UI.
func SubscriptionMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/auth/") ||
			strings.HasPrefix(path, "/public/") ||
			strings.HasPrefix(path, "/subscription/") ||
			path == "/health" {
			c.Next()
			return
		}

		// Allow reading restaurant profile when expired (Home paywall needs plan details).
		if c.Request.Method == http.MethodGet && path == "/restaurants/profile" {
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

		if !restaurant.IsActive {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "restaurant_suspended",
				"message": "This restaurant account has been suspended. Contact BillGenie support.",
			})
			c.Abort()
			return
		}

		// Closed / holiday mode — admins keep operating; everyone else is blocked.
		if restaurant.IsClosed {
			roleVal, _ := c.Get("role")
			role, _ := roleVal.(string)
			if role != "admin" {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "restaurant_closed",
					"message": "Restaurant is closed. Contact the owner to reopen.",
				})
				c.Abort()
				return
			}
		}

		// Check if subscription has expired or payment is pending
		if services.IsSubscriptionAccessBlocked(&restaurant) {
			cfg := services.ParseStoredSubscriptionConfig(&restaurant)
			daysExpired := int(time.Since(restaurant.SubscriptionEnd).Hours() / 24)
			if daysExpired < 0 {
				daysExpired = 0
			}
			log.Printf("⚠️ Subscription blocked for restaurant %s (phase=%s, %d days past end)", restaurantID, cfg.Phase, daysExpired)

			message := "Your subscription has expired. Please renew to continue using BillGenie."
			if cfg.Phase == services.SubscriptionPhasePendingPayment {
				message = "Complete payment to activate your BillGenie subscription."
			} else if cfg.Phase == services.SubscriptionPhaseTrial {
				message = "Your 15-day free trial has ended. Choose a plan and pay to continue."
			}

			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":                   "subscription_expired",
				"message":                 message,
				"subscription_end":        restaurant.SubscriptionEnd,
				"subscription_phase":      cfg.Phase,
				"requires_plan_selection": services.NeedsPlanSelection(&restaurant),
				"days_expired":            daysExpired,
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
