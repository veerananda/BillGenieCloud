package handlers

import (
	"net/http"
	"restaurant-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RestaurantHandler struct {
	db *gorm.DB
}

func NewRestaurantHandler(db *gorm.DB) *RestaurantHandler {
	return &RestaurantHandler{db: db}
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
	c.JSON(http.StatusOK, gin.H{
		"id":               restaurant.ID,
		"name":             restaurant.Name,
		"address":          restaurant.Address,
		"phone":            restaurant.Phone,
		"contact_number":   restaurant.ContactNumber,
		"email":            restaurant.Email,
		"upi_qr_code":      restaurant.UPIQRCode,
		"city":             restaurant.City,
		"cuisine":          restaurant.Cuisine,
		"is_self_service":  restaurant.IsSelfService,
		"subscription_end": restaurant.SubscriptionEnd,
	})
}

// UpdateRestaurantProfile updates the restaurant profile
func (h *RestaurantHandler) UpdateRestaurantProfile(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var input struct {
		Name          string `json:"name"`
		Address       string `json:"address"`
		ContactNumber string `json:"contact_number"`
		UPIQRCode     string `json:"upi_qr_code"`
		IsSelfService *bool  `json:"is_self_service"`
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
	if input.UPIQRCode != "" {
		restaurant.UPIQRCode = input.UPIQRCode
	}
	if input.IsSelfService != nil {
		restaurant.IsSelfService = *input.IsSelfService
	}

	if err := h.db.Save(&restaurant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update restaurant profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Restaurant profile updated successfully",
		"restaurant": restaurant,
	})
}
