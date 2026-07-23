package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MenuHandler struct {
	db *gorm.DB
}

type CreateMenuItemRequest struct {
	Name             string                   `json:"name" validate:"required"`
	Category         string                   `json:"category"`
	Description      string                   `json:"description"`
	Price            float64                  `json:"price" validate:"required,gt=0"`
	CostPrice        float64                  `json:"cost_price"`
	IsVeg            bool                     `json:"is_veg"`
	ReadilyAvailable bool                     `json:"readily_available"`
	IsTaxable        *bool                    `json:"is_taxable"`
	Variants         []services.VariantInput  `json:"variants"`
}

type UpdateMenuItemRequest struct {
	Name             string                   `json:"name"`
	Category         string                   `json:"category"`
	Description      string                   `json:"description"`
	Price            float64                  `json:"price"`
	CostPrice        float64                  `json:"cost_price"`
	IsVeg            *bool                    `json:"is_veg"`
	IsAvailable      *bool                    `json:"is_available"`
	ReadilyAvailable *bool                    `json:"readily_available"`
	IsTaxable        *bool                    `json:"is_taxable"`
	Variants         *[]services.VariantInput `json:"variants"`
}

// NewMenuHandler creates a new menu handler
func NewMenuHandler(db *gorm.DB) *MenuHandler {
	return &MenuHandler{db: db}
}

func (h *MenuHandler) getRequestUser(c *gin.Context) (*models.User, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return nil, errors.New("user not found in context")
	}
	var user models.User
	if err := h.db.Where("id = ?", userID.(string)).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func isAvailabilityOnlyUpdate(req UpdateMenuItemRequest) bool {
	return req.IsAvailable != nil &&
		req.Name == "" &&
		req.Category == "" &&
		req.Description == "" &&
		req.Price <= 0 &&
		req.CostPrice <= 0 &&
		req.IsVeg == nil &&
		req.ReadilyAvailable == nil &&
		req.IsTaxable == nil &&
		req.Variants == nil
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
	user, err := h.getRequestUser(c)
	if err != nil || !services.UserCanManageMenu(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions for menu management"})
		return
	}

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
		IsVeg:            req.IsVeg,
		IsAvailable:      true,
		ReadilyAvailable: req.ReadilyAvailable,
		IsTaxable:        true,
	}
	if req.IsTaxable != nil {
		menuItem.IsTaxable = *req.IsTaxable
	}

	if err := h.db.Create(menuItem).Error; err != nil {
		log.Printf("❌ Menu item creation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	variants, err := services.SyncMenuItemVariants(h.db, *menuItem, req.Variants)
	if err != nil {
		log.Printf("❌ Menu item variant sync failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	menuItem.Variants = variants
	_ = h.db.Where("id = ?", menuItem.ID).First(menuItem).Error
	menuItem.Variants = variants

	log.Printf("✅ Menu item created: %s (ID: %s)", menuItem.Name, menuItem.ID)

	if globalHub != nil {
		BroadcastMenuUpdate(globalHub, restaurantID.(string), "created", menuItem, "")
	}

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

	if err := query.Preload("Variants", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, created_at ASC")
	}).Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		log.Printf("❌ Menu retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for i := range items {
		if len(items[i].Variants) == 0 {
			_ = services.EnsureDefaultMenuItemVariant(h.db, items[i])
			_ = h.db.Where("menu_item_id = ?", items[i].ID).Order("sort_order ASC, created_at ASC").Find(&items[i].Variants).Error
		}
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
		Preload("Variants", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, created_at ASC")
		}).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "menu item not found"})
			return
		}
		log.Printf("❌ Menu item retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(item.Variants) == 0 {
		_ = services.EnsureDefaultMenuItemVariant(h.db, item)
		_ = h.db.Where("menu_item_id = ?", item.ID).Order("sort_order ASC, created_at ASC").Find(&item.Variants).Error
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
	user, err := h.getRequestUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

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

	if !services.UserCanManageMenu(user) {
		if user.Role != "manager" || !isAvailabilityOnlyUpdate(req) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions for menu management"})
			return
		}
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
	if req.ReadilyAvailable != nil {
		updates["readily_available"] = *req.ReadilyAvailable
	}
	if req.IsTaxable != nil {
		updates["is_taxable"] = *req.IsTaxable
	}

	if err := h.db.Model(&item).Updates(updates).Error; err != nil {
		log.Printf("❌ Menu item update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		First(&item).Error; err != nil {
		log.Printf("❌ Menu item reload failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if req.Variants != nil {
		variants, err := services.SyncMenuItemVariants(h.db, item, *req.Variants)
		if err != nil {
			log.Printf("❌ Menu item variant sync failed: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		item.Variants = variants
		_ = h.db.Where("id = ?", item.ID).First(&item).Error
		item.Variants = variants
	} else {
		_ = h.db.Where("menu_item_id = ?", item.ID).Order("sort_order ASC, created_at ASC").Find(&item.Variants).Error
		if len(item.Variants) == 0 {
			_ = services.EnsureDefaultMenuItemVariant(h.db, item)
			_ = h.db.Where("menu_item_id = ?", item.ID).Order("sort_order ASC, created_at ASC").Find(&item.Variants).Error
		}
	}

	log.Printf("✅ Menu item updated: %s", item.Name)

	if globalHub != nil {
		BroadcastMenuUpdate(globalHub, restaurantID.(string), "updated", &item, "")
	}

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
	user, err := h.getRequestUser(c)
	if err != nil || !services.UserCanManageMenu(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions for menu management"})
		return
	}

	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	menuItemID := c.Param("menu_item_id")

	tx := h.db.Begin()
	if err := tx.Where("menu_item_id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		Delete(&models.MenuItemVariant{}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Menu item variant deletion failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		Delete(&models.MenuItem{}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Menu item deletion failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Menu item deleted: %s", menuItemID)

	if globalHub != nil {
		BroadcastMenuUpdate(globalHub, restaurantID.(string), "deleted", nil, menuItemID)
	}

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
	user, err := h.getRequestUser(c)
	if err != nil || !services.UserCanToggleMenuAvailability(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions to change item availability"})
		return
	}

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

	item.IsAvailable = newAvailability

	log.Printf("✅ Menu item availability toggled: %s -> %v", item.Name, newAvailability)

	if globalHub != nil {
		BroadcastMenuUpdate(globalHub, restaurantID.(string), "updated", &item, "")
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Menu item availability toggled",
		"menu_item": gin.H{
			"id":        item.ID,
			"name":      item.Name,
			"available": newAvailability,
		},
	})
}
