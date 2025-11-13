package handlers

import (
	"log"
	"net/http"
	"strconv"

	"restaurant-api/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PublicHandler struct {
	db *gorm.DB
}

// NewPublicHandler creates a new public handler
func NewPublicHandler(db *gorm.DB) *PublicHandler {
	return &PublicHandler{db: db}
}

// GetPublicMenu retrieves menu items for a restaurant (public access)
// @Summary Get public menu
// @Description Get menu items for a specific restaurant (no authentication required)
// @Produce json
// @Param restaurant_id query string true "Restaurant ID"
// @Param category query string false "Filter by category"
// @Param available query string false "Filter by availability (true/false)"
// @Param limit query int false "Limit results (max 100)" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /public/menu [get]
func (h *PublicHandler) GetPublicMenu(c *gin.Context) {
	restaurantID := c.Query("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restaurant_id is required"})
		return
	}

	category := c.DefaultQuery("category", "")
	availableStr := c.DefaultQuery("available", "true") // Default to showing only available items
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	query := h.db.Where("restaurant_id = ?", restaurantID)

	if category != "" {
		query = query.Where("category = ?", category)
	}

	if availableStr != "" {
		available := availableStr == "true"
		query = query.Where("is_available = ?", available)
	}

	var items []models.MenuItem
	var total int64

	if err := query.Model(&models.MenuItem{}).Count(&total).Error; err != nil {
		log.Printf("❌ Public menu count failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve menu"})
		return
	}

	if err := query.Order("category ASC, name ASC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		log.Printf("❌ Public menu retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve menu"})
		return
	}

	log.Printf("✅ Public menu retrieved: %d items for restaurant %s", len(items), restaurantID)

	c.JSON(http.StatusOK, gin.H{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetPublicMenuItem retrieves a single menu item (public access)
// @Summary Get public menu item
// @Description Get details of a specific menu item (no authentication required)
// @Produce json
// @Param restaurant_id query string true "Restaurant ID"
// @Param menu_item_id path string true "Menu Item ID"
// @Success 200 {object} models.MenuItem
// @Router /public/menu/:menu_item_id [get]
func (h *PublicHandler) GetPublicMenuItem(c *gin.Context) {
	restaurantID := c.Query("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restaurant_id is required"})
		return
	}

	menuItemID := c.Param("menu_item_id")
	if menuItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "menu_item_id is required"})
		return
	}

	var item models.MenuItem
	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "menu item not found"})
			return
		}
		log.Printf("❌ Public menu item retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve menu item"})
		return
	}

	log.Printf("✅ Public menu item retrieved: %s", item.Name)

	c.JSON(http.StatusOK, item)
}

// GetPublicRestaurant retrieves restaurant info (public access)
// @Summary Get public restaurant info
// @Description Get restaurant details (no authentication required)
// @Produce json
// @Param restaurant_id query string true "Restaurant ID"
// @Success 200 {object} models.Restaurant
// @Router /public/restaurant [get]
func (h *PublicHandler) GetPublicRestaurant(c *gin.Context) {
	restaurantID := c.Query("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restaurant_id is required"})
		return
	}

	var restaurant models.Restaurant
	if err := h.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "restaurant not found"})
			return
		}
		log.Printf("❌ Public restaurant retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve restaurant"})
		return
	}

	log.Printf("✅ Public restaurant info retrieved: %s", restaurant.Name)

	// Return only public information
	c.JSON(http.StatusOK, gin.H{
		"id":      restaurant.ID,
		"name":    restaurant.Name,
		"address": restaurant.Address,
		"phone":   restaurant.Phone,
		"email":   restaurant.Email,
	})
}
