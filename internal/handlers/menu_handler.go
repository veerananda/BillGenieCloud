package handlers

import (
	"log"
	"net/http"
	"strconv"

	"restaurant-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MenuHandler struct {
	db *gorm.DB
}

type CreateMenuItemRequest struct {
	Name        string  `json:"name" validate:"required"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	CostPrice   float64 `json:"cost_price"`
	IsVeg       bool    `json:"is_veg"`
}

type UpdateMenuItemRequest struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	CostPrice   float64 `json:"cost_price"`
	IsVeg       *bool   `json:"is_veg"`
	IsAvailable *bool   `json:"is_available"`
}

// NewMenuHandler creates a new menu handler
func NewMenuHandler(db *gorm.DB) *MenuHandler {
	return &MenuHandler{db: db}
}

// CreateMenuItem creates a new menu item
// @Summary Create menu item
// @Description Add new item to restaurant menu
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body CreateMenuItemRequest true "Menu item data"
// @Success 201 {object} map[string]interface{}
// @Router /menu [post]
func (h *MenuHandler) CreateMenuItem(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req CreateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	menuItem := &models.MenuItem{
		ID:           uuid.New().String(),
		RestaurantID: restaurantID.(string),
		Name:         req.Name,
		Category:     req.Category,
		Description:  req.Description,
		Price:        req.Price,
		CostPrice:    req.CostPrice,
		IsVeg:        req.IsVeg,
		IsAvailable:  true,
	}

	if err := h.db.Create(menuItem).Error; err != nil {
		log.Printf("❌ Menu item creation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu item created: %s (ID: %s)", menuItem.Name, menuItem.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":   "Menu item created successfully",
		"menu_item": menuItem,
	})
}

// GetMenuItems retrieves all menu items
// @Summary Get menu
// @Description Get all menu items for restaurant
// @Security ApiKeyAuth
// @Produce json
// @Param category query string false "Filter by category"
// @Param available query bool false "Filter by availability"
// @Param limit query int false "Items per page (default: 50)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Success 200 {object} map[string]interface{}
// @Router /menu [get]
func (h *MenuHandler) GetMenuItems(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	category := c.DefaultQuery("category", "")
	availableStr := c.DefaultQuery("available", "")
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
		log.Printf("❌ Menu count failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := query.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		log.Printf("❌ Menu retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu items retrieved: %d items", len(items))

	c.JSON(http.StatusOK, gin.H{
		"menu_items": items,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetMenuItem retrieves a specific menu item
// @Summary Get menu item
// @Description Get details of a specific menu item
// @Security ApiKeyAuth
// @Produce json
// @Param menu_item_id path string true "Menu Item ID"
// @Success 200 {object} map[string]interface{}
// @Router /menu/:menu_item_id [get]
func (h *MenuHandler) GetMenuItem(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	menuItemID := c.Param("menu_item_id")

	var item models.MenuItem
	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "menu item not found"})
			return
		}
		log.Printf("❌ Menu item retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu item retrieved: %s", item.Name)

	c.JSON(http.StatusOK, gin.H{
		"menu_item": item,
	})
}

// UpdateMenuItem updates a menu item
// @Summary Update menu item
// @Description Update menu item details
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param menu_item_id path string true "Menu Item ID"
// @Param request body UpdateMenuItemRequest true "Update data"
// @Success 200 {object} map[string]interface{}
// @Router /menu/:menu_item_id [put]
func (h *MenuHandler) UpdateMenuItem(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	menuItemID := c.Param("menu_item_id")

	var item models.MenuItem
	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "menu item not found"})
			return
		}
		log.Printf("❌ Menu item retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req UpdateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Price > 0 {
		updates["price"] = req.Price
	}
	if req.CostPrice > 0 {
		updates["cost_price"] = req.CostPrice
	}
	if req.IsVeg != nil {
		updates["is_veg"] = *req.IsVeg
	}
	if req.IsAvailable != nil {
		updates["is_available"] = *req.IsAvailable
	}

	if err := h.db.Model(&item).Updates(updates).Error; err != nil {
		log.Printf("❌ Menu item update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu item updated: %s", item.Name)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Menu item updated successfully",
		"menu_item": item,
	})
}

// DeleteMenuItem deletes a menu item
// @Summary Delete menu item
// @Description Remove item from menu
// @Security ApiKeyAuth
// @Produce json
// @Param menu_item_id path string true "Menu Item ID"
// @Success 200 {object} map[string]interface{}
// @Router /menu/:menu_item_id [delete]
func (h *MenuHandler) DeleteMenuItem(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	menuItemID := c.Param("menu_item_id")

	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		Delete(&models.MenuItem{}).Error; err != nil {
		log.Printf("❌ Menu item deletion failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu item deleted: %s", menuItemID)

	c.JSON(http.StatusOK, gin.H{
		"message":      "Menu item deleted successfully",
		"menu_item_id": menuItemID,
	})
}

// ToggleAvailability toggles menu item availability
// @Summary Toggle availability
// @Description Mark item as available/unavailable
// @Security ApiKeyAuth
// @Produce json
// @Param menu_item_id path string true "Menu Item ID"
// @Success 200 {object} map[string]interface{}
// @Router /menu/:menu_item_id/toggle [put]
func (h *MenuHandler) ToggleAvailability(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	menuItemID := c.Param("menu_item_id")

	var item models.MenuItem
	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "menu item not found"})
			return
		}
		log.Printf("❌ Menu item retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newAvailability := !item.IsAvailable
	if err := h.db.Model(&item).Update("is_available", newAvailability).Error; err != nil {
		log.Printf("❌ Availability toggle failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu item availability toggled: %s -> %v", item.Name, newAvailability)

	c.JSON(http.StatusOK, gin.H{
		"message": "Menu item availability toggled",
		"menu_item": gin.H{
			"id":        item.ID,
			"name":      item.Name,
			"available": newAvailability,
		},
	})
}
