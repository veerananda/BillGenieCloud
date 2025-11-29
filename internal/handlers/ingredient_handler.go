package handlers

import (
	"log"
	"net/http"

	"restaurant-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type IngredientHandler struct {
	db *gorm.DB
}

type CreateIngredientRequest struct {
	Name         string  `json:"name" binding:"required"`
	Unit         string  `json:"unit" binding:"required"`
	CurrentStock float64 `json:"current_stock"`
	FullStock    float64 `json:"full_stock"`
}

type UpdateIngredientRequest struct {
	Name         string  `json:"name"`
	Unit         string  `json:"unit"`
	CurrentStock float64 `json:"current_stock"`
	FullStock    float64 `json:"full_stock"`
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
		ID:           uuid.New().String(),
		RestaurantID: restaurantID.(string),
		Name:         req.Name,
		Unit:         req.Unit,
		CurrentStock: req.CurrentStock,
		FullStock:    req.FullStock,
	}

	if err := h.db.Create(ingredient).Error; err != nil {
		log.Printf("❌ Ingredient creation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Ingredient created: %s (ID: %s)", ingredient.Name, ingredient.ID)

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

	// Update fields
	if req.Name != "" {
		ingredient.Name = req.Name
	}
	if req.Unit != "" {
		ingredient.Unit = req.Unit
	}
	ingredient.CurrentStock = req.CurrentStock
	ingredient.FullStock = req.FullStock

	if err := h.db.Save(&ingredient).Error; err != nil {
		log.Printf("❌ Ingredient update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Ingredient updated: %s (ID: %s)", ingredient.Name, ingredient.ID)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Ingredient updated successfully",
		"ingredient": ingredient,
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
