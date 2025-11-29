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

type InventoryHandler struct {
	db *gorm.DB
}

type UpdateInventoryRequest struct {
	MenuItemID int     `json:"menu_item_id" validate:"required,min=1"`
	Quantity   float64 `json:"quantity" validate:"required,min=0"`
	Unit       string  `json:"unit"`
	MinLevel   float64 `json:"min_level"`
	MaxLevel   float64 `json:"max_level"`
	Notes      string  `json:"notes"`
}

// NewInventoryHandler creates a new inventory handler
func NewInventoryHandler(db *gorm.DB) *InventoryHandler {
	return &InventoryHandler{db: db}
}

// GetInventory retrieves inventory levels
// @Summary Get inventory
// @Description Get all inventory items for restaurant
// @Security ApiKeyAuth
// @Produce json
// @Param limit query int false "Items per page (default: 20)"
// @Param offset query int false "Pagination offset (default: 0)"
// @Param low_stock query bool false "Show only low stock items"
// @Success 200 {object} map[string]interface{}
// @Router /inventory [get]
func (h *InventoryHandler) GetInventory(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	lowStock := c.DefaultQuery("low_stock", "") == "true"

	if limit > 100 {
		limit = 100
	}

	query := h.db.Where("restaurant_id = ?", restaurantID)

	if lowStock {
		query = query.Where("quantity < min_level")
	}

	var inventory []models.Inventory
	var total int64

	if err := query.Model(&models.Inventory{}).Count(&total).Error; err != nil {
		log.Printf("❌ Inventory count failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := query.Preload("MenuItem").
		Limit(limit).
		Offset(offset).
		Find(&inventory).Error; err != nil {
		log.Printf("❌ Inventory retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Inventory retrieved: %d items", len(inventory))

	c.JSON(http.StatusOK, gin.H{
		"inventory": inventory,
		"total":     total,
		"limit":     limit,
		"offset":    offset,
	})
}

// UpdateInventory updates stock level
// @Summary Update inventory
// @Description Update inventory quantity for a menu item
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param menu_item_id path int true "Menu Item ID"
// @Param request body UpdateInventoryRequest true "Update data"
// @Success 200 {object} map[string]interface{}
// @Router /inventory/:menu_item_id [put]
func (h *InventoryHandler) UpdateInventory(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	menuItemID := c.Param("menu_item_id")
	if menuItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "menu_item_id is required"})
		return
	}

	var req UpdateInventoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find inventory record
	var inventory models.Inventory
	if err := h.db.Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItemID).
		First(&inventory).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new inventory record if doesn't exist
			inventory = models.Inventory{
				ID:           uuid.New().String(),
				RestaurantID: restaurantID.(string),
				MenuItemID:   menuItemID,
				Quantity:     req.Quantity,
				Unit:         req.Unit,
				MinLevel:     req.MinLevel,
				MaxLevel:     req.MaxLevel,
			}
			if err := h.db.Create(&inventory).Error; err != nil {
				log.Printf("❌ Inventory creation failed: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else {
			log.Printf("❌ Inventory retrieval failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Update existing inventory
		updates := map[string]interface{}{
			"quantity":  req.Quantity,
			"min_level": req.MinLevel,
			"max_level": req.MaxLevel,
		}
		if req.Unit != "" {
			updates["unit"] = req.Unit
		}

		if err := h.db.Model(&inventory).Updates(updates).Error; err != nil {
			log.Printf("❌ Inventory update failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	log.Printf("✅ Inventory updated: Menu Item %s, Quantity: %.2f", menuItemID, req.Quantity)

	c.JSON(http.StatusOK, gin.H{
		"message": "Inventory updated successfully",
		"inventory": gin.H{
			"id":        inventory.ID,
			"quantity":  inventory.Quantity,
			"unit":      inventory.Unit,
			"min_level": inventory.MinLevel,
			"max_level": inventory.MaxLevel,
		},
	})
}

// DeductInventory deducts stock (used when order is created)
// @Summary Deduct inventory
// @Description Deduct stock after order
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Deduction data"
// @Success 200 {object} map[string]interface{}
// @Router /inventory/deduct [post]
func (h *InventoryHandler) DeductInventory(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	menuItemIDFloat, ok := req["menu_item_id"].(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid menu_item_id"})
		return
	}
	menuItemID := int(menuItemIDFloat)

	quantityFloat, ok := req["quantity"].(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quantity"})
		return
	}

	// Deduct inventory
	if err := h.db.Model(&models.Inventory{}).
		Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItemID).
		Update("quantity", gorm.Expr("quantity - ?", quantityFloat)).Error; err != nil {
		log.Printf("❌ Inventory deduction failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Inventory deducted: Menu Item %d, Quantity: %.2f", menuItemID, quantityFloat)

	c.JSON(http.StatusOK, gin.H{
		"message": "Inventory deducted successfully",
		"deducted": gin.H{
			"menu_item_id": menuItemID,
			"quantity":     quantityFloat,
		},
	})
}

// RestockInventory adds stock back
// @Summary Restock inventory
// @Description Add stock back (e.g., when order is cancelled)
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Restock data"
// @Success 200 {object} map[string]interface{}
// @Router /inventory/restock [post]
func (h *InventoryHandler) RestockInventory(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	menuItemIDFloat, ok := req["menu_item_id"].(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid menu_item_id"})
		return
	}
	menuItemID := int(menuItemIDFloat)

	quantityFloat, ok := req["quantity"].(float64)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quantity"})
		return
	}

	// Restock inventory
	if err := h.db.Model(&models.Inventory{}).
		Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItemID).
		Update("quantity", gorm.Expr("quantity + ?", quantityFloat)).Error; err != nil {
		log.Printf("❌ Inventory restock failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Inventory restocked: Menu Item %d, Quantity: %.2f", menuItemID, quantityFloat)

	c.JSON(http.StatusOK, gin.H{
		"message": "Inventory restocked successfully",
		"restocked": gin.H{
			"menu_item_id": menuItemID,
			"quantity":     quantityFloat,
		},
	})
}

// GetLowStockAlert retrieves items with low stock
// @Summary Get low stock alerts
// @Description Get inventory items below minimum level
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /inventory/alerts [get]
func (h *InventoryHandler) GetLowStockAlert(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var lowStockItems []models.Inventory

	if err := h.db.Where("restaurant_id = ? AND quantity < min_level", restaurantID).
		Preload("MenuItem").
		Find(&lowStockItems).Error; err != nil {
		log.Printf("❌ Low stock alert retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Low stock alerts retrieved: %d items below minimum", len(lowStockItems))

	c.JSON(http.StatusOK, gin.H{
		"low_stock_items": lowStockItems,
		"count":           len(lowStockItems),
	})
}
