package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IngredientHandler struct {
	db *gorm.DB
}

type CreateIngredientRequest struct {
	Name          string  `json:"name" binding:"required"`
	Unit          string  `json:"unit" binding:"required"`
	CurrentStock  float64 `json:"current_stock"`
	FullStock     float64 `json:"full_stock"`
	AlertQuantity float64 `json:"alert_quantity"`
}

type UpdateIngredientRequest struct {
	Name          *string  `json:"name"`
	Unit          *string  `json:"unit"`
	CurrentStock  *float64 `json:"current_stock"`
	FullStock     *float64 `json:"full_stock"`
	AlertQuantity *float64 `json:"alert_quantity"`
}

type BulkUpdateIngredientItem struct {
	IngredientID  string   `json:"ingredient_id" binding:"required"`
	AlertQuantity *float64 `json:"alert_quantity"`
	FullStock     *float64 `json:"full_stock"`
}

type BulkUpdateIngredientsRequest struct {
	Items []BulkUpdateIngredientItem `json:"items" binding:"required,min=1,dive"`
}

type RestockIngredientRequest struct {
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
}

// RestockItem is one line in a bulk restock request.
type RestockItem struct {
	IngredientID string  `json:"ingredient_id" binding:"required"`
	Quantity     float64 `json:"quantity" binding:"required,gt=0"`
}

// RestockIngredientsRequest refills multiple ingredients in one call.
// Zero / missing quantities should be filtered out by the client before sending.
type RestockIngredientsRequest struct {
	Items []RestockItem `json:"items" binding:"required,min=1,dive"`
}

// NewIngredientHandler creates a new ingredient handler
func NewIngredientHandler(db *gorm.DB) *IngredientHandler {
	return &IngredientHandler{db: db}
}

// ListIngredients retrieves all ingredients for a restaurant
// @Summary Get ingredients
// @Description Get all ingredients for restaurant
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /ingredients [get]
func (h *IngredientHandler) ListIngredients(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var ingredients []models.Ingredient
	if err := h.db.Where("restaurant_id = ?", restaurantID).
		Order("name ASC").
		Find(&ingredients).Error; err != nil {
		log.Printf("❌ Ingredients retrieval failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Ingredients retrieved: %d items", len(ingredients))

	c.JSON(http.StatusOK, gin.H{
		"ingredients": ingredients,
		"total":       len(ingredients),
	})
}

// CreateIngredient creates a new ingredient
// @Summary Create ingredient
// @Description Add new ingredient to inventory
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body CreateIngredientRequest true "Ingredient data"
// @Success 201 {object} map[string]interface{}
// @Router /ingredients [post]
func (h *IngredientHandler) CreateIngredient(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req CreateIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ingredient := &models.Ingredient{
		ID:            uuid.New().String(),
		RestaurantID:  restaurantID.(string),
		Name:          req.Name,
		Unit:          req.Unit,
		CurrentStock:  req.CurrentStock,
		FullStock:     req.FullStock,
		AlertQuantity: req.AlertQuantity,
	}

	if err := h.db.Create(ingredient).Error; err != nil {
		log.Printf("❌ Ingredient creation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Ingredient created: %s (ID: %s)", ingredient.Name, ingredient.ID)

	if globalHub != nil {
		BroadcastIngredientInventoryUpdate(globalHub, restaurantID.(string), *ingredient)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Ingredient created successfully",
		"ingredient": ingredient,
	})
}

// UpdateIngredient updates an existing ingredient
// @Summary Update ingredient
// @Description Update ingredient details
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param ingredient_id path string true "Ingredient ID"
// @Param request body UpdateIngredientRequest true "Update data"
// @Success 200 {object} map[string]interface{}
// @Router /ingredients/:ingredient_id [put]
func (h *IngredientHandler) UpdateIngredient(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	ingredientID := c.Param("ingredient_id")

	var req UpdateIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find ingredient
	var ingredient models.Ingredient
	if err := h.db.Where("id = ? AND restaurant_id = ?", ingredientID, restaurantID).
		First(&ingredient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "ingredient not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update fields (omit unset pointer fields so stock refill / alert edits stay independent)
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name != "" {
			ingredient.Name = name
		}
	}
	if req.Unit != nil {
		unit := strings.TrimSpace(*req.Unit)
		if unit != "" {
			ingredient.Unit = unit
		}
	}
	if req.CurrentStock != nil {
		if *req.CurrentStock < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "current_stock must be >= 0"})
			return
		}
		ingredient.CurrentStock = *req.CurrentStock
	}
	if req.FullStock != nil {
		if *req.FullStock < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "full_stock must be >= 0"})
			return
		}
		ingredient.FullStock = *req.FullStock
	}
	if req.AlertQuantity != nil {
		if *req.AlertQuantity < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "alert_quantity must be >= 0"})
			return
		}
		ingredient.AlertQuantity = *req.AlertQuantity
	}

	if err := h.db.Save(&ingredient).Error; err != nil {
		log.Printf("❌ Ingredient update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := syncRecipeDenormalizedNames(h.db, restaurantID.(string), ingredient.ID, ingredient.Name, ingredient.Unit); err != nil {
		log.Printf("⚠️ Failed to sync recipe names after ingredient update: %v", err)
	}

	log.Printf("✅ Ingredient updated: %s (ID: %s)", ingredient.Name, ingredient.ID)

	if globalHub != nil {
		BroadcastIngredientInventoryUpdate(globalHub, restaurantID.(string), ingredient)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Ingredient updated successfully",
		"ingredient": ingredient,
	})
}

// BulkUpdateIngredients updates alert (and optional full) quantities for many ingredients.
// @Summary Bulk update ingredients
// @Description Update alert_quantity (and optionally full_stock) for multiple ingredients
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body BulkUpdateIngredientsRequest true "Bulk update payload"
// @Success 200 {object} map[string]interface{}
// @Router /ingredients/bulk [put]
func (h *IngredientHandler) BulkUpdateIngredients(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req BulkUpdateIngredientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated := make([]models.Ingredient, 0, len(req.Items))
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range req.Items {
			if item.AlertQuantity == nil && item.FullStock == nil {
				return fmt.Errorf("ingredient %s: provide alert_quantity and/or full_stock", item.IngredientID)
			}
			if item.AlertQuantity != nil && *item.AlertQuantity < 0 {
				return fmt.Errorf("ingredient %s: alert_quantity must be >= 0", item.IngredientID)
			}
			if item.FullStock != nil && *item.FullStock < 0 {
				return fmt.Errorf("ingredient %s: full_stock must be >= 0", item.IngredientID)
			}

			var ingredient models.Ingredient
			if err := tx.Where("id = ? AND restaurant_id = ?", item.IngredientID, restaurantID).
				First(&ingredient).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("ingredient not found: %s", item.IngredientID)
				}
				return err
			}

			updates := map[string]interface{}{}
			if item.AlertQuantity != nil {
				updates["alert_quantity"] = *item.AlertQuantity
				ingredient.AlertQuantity = *item.AlertQuantity
			}
			if item.FullStock != nil {
				updates["full_stock"] = *item.FullStock
				ingredient.FullStock = *item.FullStock
			}

			if err := tx.Model(&ingredient).Updates(updates).Error; err != nil {
				return err
			}
			updated = append(updated, ingredient)
		}
		return nil
	})
	if err != nil {
		log.Printf("❌ Bulk ingredient update failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if globalHub != nil {
		BroadcastIngredientInventoryUpdates(globalHub, restaurantID.(string), updated)
	}

	log.Printf("✅ Bulk updated %d ingredients for restaurant %s", len(updated), restaurantID)
	c.JSON(http.StatusOK, gin.H{
		"message":     "Ingredients updated successfully",
		"ingredients": updated,
		"total":       len(updated),
	})
}

// DeleteIngredient deletes an ingredient
// @Summary Delete ingredient
// @Description Remove ingredient from inventory
// @Security ApiKeyAuth
// @Produce json
// @Param ingredient_id path string true "Ingredient ID"
// @Success 200 {object} map[string]interface{}
// @Router /ingredients/:ingredient_id [delete]
func (h *IngredientHandler) DeleteIngredient(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	ingredientID := c.Param("ingredient_id")

	if err := h.db.Where("restaurant_id = ? AND ingredient_id = ?", restaurantID, ingredientID).
		Delete(&models.MenuItemIngredient{}).Error; err != nil {
		log.Printf("❌ Failed to remove recipe lines for ingredient: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := h.db.Where("id = ? AND restaurant_id = ?", ingredientID, restaurantID).
		Delete(&models.Ingredient{})

	if result.Error != nil {
		log.Printf("❌ Ingredient deletion failed: %v", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "ingredient not found"})
		return
	}

	log.Printf("✅ Ingredient deleted: %s", ingredientID)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Ingredient deleted successfully",
		"ingredient_id": ingredientID,
	})
}

// SyncFromRecipes ensures inventory ingredient rows exist for all recipe ingredients (no duplicates).
func (h *IngredientHandler) SyncFromRecipes(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	if err := syncIngredientsFromRecipes(h.db, restaurantID.(string)); err != nil {
		log.Printf("❌ Sync from recipes failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var ingredients []models.Ingredient
	if err := h.db.Where("restaurant_id = ?", restaurantID).
		Order("name ASC").
		Find(&ingredients).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Ingredients synced from recipes",
		"ingredients": ingredients,
		"total":       len(ingredients),
	})
}

func userCanRestockFromContext(c *gin.Context, db *gorm.DB) bool {
	userID, ok := c.Get("user_id")
	if !ok {
		return false
	}
	var user models.User
	if err := db.Where("id = ?", userID.(string)).First(&user).Error; err != nil {
		return false
	}
	return services.UserCanRestockInventory(&user)
}

var (
	errNoValidRestockQuantities = errors.New("no valid restock quantities")
	errIngredientNotFound       = errors.New("ingredient not found")
)

// applyRestockItems adds stock for each item in a single transaction and returns updated rows.
func (h *IngredientHandler) applyRestockItems(restaurantID string, items []RestockItem) ([]models.Ingredient, error) {
	type qtyLine struct {
		id  string
		qty float64
	}
	ordered := make([]qtyLine, 0, len(items))
	indexByID := make(map[string]int, len(items))
	for _, item := range items {
		id := item.IngredientID
		if id == "" || item.Quantity <= 0 {
			continue
		}
		if idx, ok := indexByID[id]; ok {
			ordered[idx].qty += item.Quantity
			continue
		}
		indexByID[id] = len(ordered)
		ordered = append(ordered, qtyLine{id: id, qty: item.Quantity})
	}
	if len(ordered) == 0 {
		return nil, errNoValidRestockQuantities
	}

	updated := make([]models.Ingredient, 0, len(ordered))
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, line := range ordered {
			var ingredient models.Ingredient
			if err := tx.Where("id = ? AND restaurant_id = ?", line.id, restaurantID).
				First(&ingredient).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return fmt.Errorf("%w: %s", errIngredientNotFound, line.id)
				}
				return err
			}
			if err := tx.Model(&ingredient).
				Update("current_stock", gorm.Expr("current_stock + ?", line.qty)).Error; err != nil {
				return err
			}
			if err := tx.Where("id = ?", line.id).First(&ingredient).Error; err != nil {
				return err
			}
			updated = append(updated, ingredient)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// RestockIngredient adds quantity to current_stock (refill) for one ingredient.
// Prefer POST /ingredients/restock for bulk refills.
func (h *IngredientHandler) RestockIngredient(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	if !userCanRestockFromContext(c, h.db) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission to restock inventory"})
		return
	}

	ingredientID := c.Param("ingredient_id")
	var req RestockIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.applyRestockItems(restaurantID.(string), []RestockItem{
		{IngredientID: ingredientID, Quantity: req.Quantity},
	})
	if err != nil {
		log.Printf("❌ Ingredient restock failed: %v", err)
		if errors.Is(err, errIngredientNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(updated) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no stock updated"})
		return
	}

	ingredient := updated[0]
	log.Printf("✅ Ingredient restocked: %s +%.3f %s (now %.3f)",
		ingredient.Name, req.Quantity, ingredient.Unit, ingredient.CurrentStock)

	if globalHub != nil {
		BroadcastIngredientInventoryUpdate(globalHub, restaurantID.(string), ingredient)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Stock added successfully",
		"ingredient":   ingredient,
		"ingredients":  updated,
		"added":        req.Quantity,
		"updated":      len(updated),
	})
}

// RestockIngredients bulk-refills multiple ingredients in one request.
func (h *IngredientHandler) RestockIngredients(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	if !userCanRestockFromContext(c, h.db) {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission to restock inventory"})
		return
	}

	var req RestockIngredientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.applyRestockItems(restaurantID.(string), req.Items)
	if err != nil {
		log.Printf("❌ Bulk ingredient restock failed: %v", err)
		if errors.Is(err, errNoValidRestockQuantities) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, errIngredientNotFound) || strings.Contains(err.Error(), "ingredient not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Bulk restock: %d ingredients updated for restaurant %s", len(updated), restaurantID)

	if globalHub != nil {
		for _, ingredient := range updated {
			BroadcastIngredientInventoryUpdate(globalHub, restaurantID.(string), ingredient)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Stock added successfully",
		"ingredients": updated,
		"updated":     len(updated),
	})
}
