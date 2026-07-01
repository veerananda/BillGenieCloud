package handlers

import (
	"log"
	"net/http"
	"strings"

	"restaurant-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MenuItemIngredientHandler struct {
	db *gorm.DB
}

func NewMenuItemIngredientHandler(db *gorm.DB) *MenuItemIngredientHandler {
	return &MenuItemIngredientHandler{db: db}
}

type SetMenuItemIngredientsRequest struct {
	Ingredients []RecipeIngredientInput `json:"ingredients" binding:"required"`
}

// ListMenuItemIngredients returns recipe lines for the restaurant (optional menu_item_id filter).
func (h *MenuItemIngredientHandler) ListMenuItemIngredients(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	query := h.db.Where("restaurant_id = ?", restaurantID)
	if menuItemID := strings.TrimSpace(c.Query("menu_item_id")); menuItemID != "" {
		query = query.Where("menu_item_id = ?", menuItemID)
	}

	var items []models.MenuItemIngredient
	if err := query.Order("menu_item_id ASC, name ASC").Find(&items).Error; err != nil {
		log.Printf("❌ Menu item ingredients retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"menu_item_ingredients": items,
		"total":                 len(items),
	})
}

// SetMenuItemIngredients replaces all recipe lines for a menu item.
func (h *MenuItemIngredientHandler) SetMenuItemIngredients(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}
	restaurantIDStr := restaurantID.(string)

	menuItemID := c.Param("menu_item_id")
	var menuItem models.MenuItem
	if err := h.db.Where("id = ? AND restaurant_id = ?", menuItemID, restaurantIDStr).First(&menuItem).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "menu item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req SetMenuItemIngredientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx := h.db.Begin()

	if err := tx.Where("restaurant_id = ? AND menu_item_id = ?", restaurantIDStr, menuItemID).
		Delete(&models.MenuItemIngredient{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	created := make([]models.MenuItemIngredient, 0, len(req.Ingredients))
	for _, ing := range req.Ingredients {
		name := strings.TrimSpace(ing.Name)
		unit := strings.TrimSpace(ing.Unit)
		if name == "" || unit == "" {
			continue
		}

		inventoryRow, err := resolveIngredientForRecipeLine(tx, restaurantIDStr, ing)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		row := models.MenuItemIngredient{
			ID:           uuid.New().String(),
			RestaurantID: restaurantIDStr,
			MenuItemID:   menuItemID,
			IngredientID: inventoryRow.ID,
			Name:         inventoryRow.Name,
			Unit:         inventoryRow.Unit,
			QuantityUsed: ing.QuantityUsed,
		}
		if err := tx.Create(&row).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		created = append(created, row)
	}

	if err := pruneOrphanIngredients(tx, restaurantIDStr); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":               "Menu item ingredients updated",
		"menu_item_ingredients": created,
	})
}
