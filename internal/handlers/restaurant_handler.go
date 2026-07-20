package handlers

import (
	"log"
	"net/http"
	"regexp"
	"restaurant-api/internal/models"
	"restaurant-api/internal/services"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var upiIDPattern = regexp.MustCompile(`^[a-zA-Z0-9.\-_]{2,256}@[a-zA-Z]{2,64}$`)

type RestaurantHandler struct {
	db          *gorm.DB
	authService *services.AuthService
}

func NewRestaurantHandler(db *gorm.DB, authService *services.AuthService) *RestaurantHandler {
	return &RestaurantHandler{db: db, authService: authService}
}

// GetRestaurantProfile retrieves the restaurant profile
func (h *RestaurantHandler) GetRestaurantProfile(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var restaurant models.Restaurant
	if err := h.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
		return
	}

	// Return profile data
	counterModes := restaurant.CounterServiceModes
	if counterModes == "" {
		counterModes = "both"
	}

	limits, _ := services.LoadSubscriptionLimits(h.db, &restaurant)
	usage, _ := services.LoadSubscriptionUsage(h.db, restaurant.ID)
	cfg := services.ParseStoredSubscriptionConfig(&restaurant)
	selection := cfg.Selection

	c.JSON(http.StatusOK, gin.H{
		"id":                         restaurant.ID,
		"name":                       restaurant.Name,
		"address":                    restaurant.Address,
		"phone":                      restaurant.Phone,
		"contact_number":             restaurant.ContactNumber,
		"email":                      restaurant.Email,
		"upi_id":                     restaurant.UPIID,
		"upi_qr_code":                restaurant.UPIQRCode,
		"city":                       restaurant.City,
		"cuisine":                    restaurant.Cuisine,
		"is_self_service":            restaurant.IsSelfService,
		"is_closed":                  restaurant.IsClosed,
		"counter_service_modes":      counterModes,
		"prices_include_gst":         restaurant.PricesIncludeGST,
		"subscription_end":           restaurant.SubscriptionEnd,
		"subscription_plan":          restaurant.SubscriptionPlan,
		"subscription_monthly_price": restaurant.SubscriptionMonthlyPrice,
		"subscription_phase":         cfg.Phase,
		"requires_plan_selection":    services.NeedsPlanSelection(&restaurant),
		"can_change_plan":            services.CanChangePlanMidCycle(&restaurant),
		"pending_selection":          cfg.PendingSelection,
		"pending_change_at":          cfg.PendingChangeAt,
		"subscription_config":        restaurant.SubscriptionConfig,
		"subscription_selection":     selection,
		"subscription_limits":        limits,
		"subscription_usage":         usage,
	})
}

// UpdateRestaurantProfile updates the restaurant profile
func (h *RestaurantHandler) UpdateRestaurantProfile(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	roleVal, _ := c.Get("role")
	role, _ := roleVal.(string)

	var input struct {
		Name                string  `json:"name"`
		Address             string  `json:"address"`
		ContactNumber       string  `json:"contact_number"`
		UPIID               *string `json:"upi_id"`
		UPIQRCode           string  `json:"upi_qr_code"`
		IsSelfService       *bool   `json:"is_self_service"`
		IsClosed            *bool   `json:"is_closed"`
		CounterServiceModes string  `json:"counter_service_modes"`
		PricesIncludeGST    *bool   `json:"prices_include_gst"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var restaurant models.Restaurant
	if err := h.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Restaurant not found"})
		return
	}

	wasClosed := restaurant.IsClosed

	// Update fields if provided
	if input.Name != "" {
		restaurant.Name = input.Name
	}
	if input.Address != "" {
		restaurant.Address = input.Address
	}
	if input.ContactNumber != "" {
		restaurant.ContactNumber = input.ContactNumber
	}
	if input.UPIID != nil {
		upiID := strings.TrimSpace(*input.UPIID)
		if upiID != "" && !upiIDPattern.MatchString(upiID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "upi_id must be a valid UPI address (e.g. name@bank)"})
			return
		}
		restaurant.UPIID = upiID
	}
	if input.UPIQRCode != "" {
		restaurant.UPIQRCode = input.UPIQRCode
	}
	if input.IsSelfService != nil {
		restaurant.IsSelfService = *input.IsSelfService
	}
	if input.IsClosed != nil {
		if role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "only the owner can open or close the restaurant"})
			return
		}
		restaurant.IsClosed = *input.IsClosed
	}
	if input.CounterServiceModes != "" {
		switch input.CounterServiceModes {
		case "both", "eat_here", "takeaway":
			restaurant.CounterServiceModes = input.CounterServiceModes
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "counter_service_modes must be both, eat_here, or takeaway"})
			return
		}
	}
	if input.PricesIncludeGST != nil {
		restaurant.PricesIncludeGST = *input.PricesIncludeGST
	}

	if err := h.db.Save(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update restaurant profile"})
		return
	}

	// Closing kicks staff/manager/chef so they cannot keep operating.
	if input.IsClosed != nil && *input.IsClosed && !wasClosed && h.authService != nil {
		revoked, err := h.authService.RevokeNonAdminSessionsForRestaurant(restaurant.ID)
		if err != nil {
			log.Printf("⚠️ Failed to revoke non-admin sessions after close: %v", err)
		} else {
			for _, uid := range revoked {
				BroadcastSessionRevoked(restaurant.ID, uid)
			}
			log.Printf("✅ Revoked %d non-admin session(s) after restaurant close", len(revoked))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Restaurant profile updated successfully",
		"restaurant": restaurant,
	})
}

// LogoutAllDevices forces every other device in the restaurant to sign out (admin only).
// POST /restaurants/logout-all-devices
func (h *RestaurantHandler) LogoutAllDevices(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}
	userID, _ := c.Get("user_id")
	roleVal, _ := c.Get("role")
	role, _ := roleVal.(string)
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only the owner can log out all devices"})
		return
	}

	authHeader := c.GetHeader("Authorization")
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authorization header"})
		return
	}
	accessToken := parts[1]

	revokedUsers, err := h.authService.RevokeRestaurantSessionsExceptCurrent(
		restaurantID.(string),
		userID.(string),
		accessToken,
	)
	if err != nil {
		log.Printf("❌ Logout all devices failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log out other devices"})
		return
	}

	for _, uid := range revokedUsers {
		BroadcastSessionRevoked(restaurantID.(string), uid)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "All other devices have been logged out",
		"revoked_users": len(revokedUsers),
	})
}
