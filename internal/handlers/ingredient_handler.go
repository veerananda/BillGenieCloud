package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"restaurant-api/internal/models"
	"restaurant-api/internal/services"
	"restaurant-api/internal/units"

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
	Unit          string   `json:"unit,omitempty"` // optional entry unit for alert/full values
}

type BulkUpdateIngredientsRequest struct {
	Items []BulkUpdateIngredientItem `json:"items" binding:"required,min=1,dive"`
}

type RestockIngredientRequest struct {
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
	Unit     string  `json:"unit,omitempty"`  // optional; defaults to ingredient inventory unit
	Price    float64 `json:"price,omitempty"` // optional purchase cost for this refill line
}

// RestockItem is one line in a bulk restock request.
type RestockItem struct {
	IngredientID string  `json:"ingredient_id" binding:"required"`
	Quantity     float64 `json:"quantity" binding:"required,gt=0"`
	Unit         string  `json:"unit,omitempty"`  // optional; defaults to ingredient inventory unit
	Price        float64 `json:"price,omitempty"` // optional purchase cost for this refill line
}

// RestockIngredientsRequest refills multiple ingredients in one call.
// Zero / missing quantities should be filtered out by the client before sending.
type RestockIngredientsRequest struct {
	Items []RestockItem `json:"items" binding:"required,min=1,dive"`
}

// DeductIngredientRequest removes stock (e.g. expired items). Admin/manager only.
type DeductIngredientRequest struct {
	IngredientID string  `json:"ingredient_id" binding:"required"`
	Quantity     float64 `json:"quantity" binding:"required,gt=0"`
	Unit         string  `json:"unit,omitempty"`
	Reason       string  `json:"reason,omitempty"` // default: expired
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

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	canonical := units.CanonicalUnit(req.Unit)
	currentStock, err := units.Convert(req.CurrentStock, req.Unit, canonical)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fullStock, err := units.Convert(req.FullStock, req.Unit, canonical)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	alertQty, err := units.Convert(req.AlertQuantity, req.Unit, canonical)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	restaurantIDStr := restaurantID.(string)

	// One inventory row per name within a unit family (e.g. onions grams/kg → onions kg).
	existing, findErr := findIngredientByNameFamily(h.db, restaurantIDStr, name, req.Unit)
	if findErr == nil {
		updates := map[string]interface{}{}
		if units.NormalizeUnit(existing.Unit) != canonical {
			updates["unit"] = canonical
			existing.Unit = canonical
		}
		if currentStock > 0 {
			existing.CurrentStock += currentStock
			updates["current_stock"] = existing.CurrentStock
		}
		if fullStock > existing.FullStock {
			existing.FullStock = fullStock
			updates["full_stock"] = fullStock
		}
		if alertQty > existing.AlertQuantity {
			existing.AlertQuantity = alertQty
			updates["alert_quantity"] = alertQty
		}
		if len(updates) > 0 {
			if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
				log.Printf("❌ Ingredient merge update failed: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			_ = syncRecipeDenormalizedNames(h.db, restaurantIDStr, existing.ID, existing.Name, existing.Unit)
		}

		log.Printf("✅ Ingredient reused (no duplicate): %s [%s] (ID: %s)", existing.Name, existing.Unit, existing.ID)
		if globalHub != nil {
			BroadcastIngredientInventoryUpdate(globalHub, restaurantIDStr, existing)
		}
		c.JSON(http.StatusOK, gin.H{
			"message":    fmt.Sprintf("%s already exists in inventory (tracked as %s); duplicate not created", existing.Name, existing.Unit),
			"ingredient": existing,
			"reused":     true,
		})
		return
	}
	if findErr != gorm.ErrRecordNotFound {
		log.Printf("❌ Ingredient lookup failed: %v", findErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": findErr.Error()})
		return
	}

	ingredient := &models.Ingredient{
		ID:            uuid.New().String(),
		RestaurantID:  restaurantIDStr,
		Name:          name,
		Unit:          canonical,
		CurrentStock:  currentStock,
		FullStock:     fullStock,
		AlertQuantity: alertQty,
	}

	if err := h.db.Create(ingredient).Error; err != nil {
		log.Printf("❌ Ingredient creation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Ingredient created: %s (ID: %s)", ingredient.Name, ingredient.ID)

	if globalHub != nil {
		BroadcastIngredientInventoryUpdate(globalHub, restaurantIDStr, *ingredient)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Ingredient created successfully",
		"ingredient": ingredient,
		"reused":     false,
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
			newCanonical := units.CanonicalUnit(unit)
			oldUnit := ingredient.Unit
			if units.NormalizeUnit(oldUnit) != newCanonical {
				if units.SameFamily(oldUnit, newCanonical) {
					if converted, convErr := units.Convert(ingredient.CurrentStock, oldUnit, newCanonical); convErr == nil {
						ingredient.CurrentStock = converted
					}
					if converted, convErr := units.Convert(ingredient.FullStock, oldUnit, newCanonical); convErr == nil {
						ingredient.FullStock = converted
					}
					if converted, convErr := units.Convert(ingredient.AlertQuantity, oldUnit, newCanonical); convErr == nil {
						ingredient.AlertQuantity = converted
					}
				}
				ingredient.Unit = newCanonical
			}
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

	// Block rename/unit change that would duplicate another row in the same unit family.
	conflict, conflictErr := findIngredientByNameFamily(h.db, restaurantID.(string), ingredient.Name, ingredient.Unit)
	if conflictErr == nil && conflict.ID != ingredient.ID {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf(
				"%s already exists in inventory (tracked as %s); cannot create a duplicate under another unit in the same family",
				conflict.Name,
				conflict.Unit,
			),
		})
		return
	}
	if conflictErr != nil && conflictErr != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": conflictErr.Error()})
		return
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
			entryUnit := strings.TrimSpace(item.Unit)
			if entryUnit == "" {
				entryUnit = ingredient.Unit
			}
			if item.AlertQuantity != nil {
				alertQty, convErr := units.Convert(*item.AlertQuantity, entryUnit, ingredient.Unit)
				if convErr != nil {
					return fmt.Errorf("ingredient %s: %w", item.IngredientID, convErr)
				}
				updates["alert_quantity"] = alertQty
				ingredient.AlertQuantity = alertQty
			}
			if item.FullStock != nil {
				fullStock, convErr := units.Convert(*item.FullStock, entryUnit, ingredient.Unit)
				if convErr != nil {
					return fmt.Errorf("ingredient %s: %w", item.IngredientID, convErr)
				}
				updates["full_stock"] = fullStock
				ingredient.FullStock = fullStock
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

func istLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*60*60+30*60)
	}
	return loc
}

func contextUserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// applyRestockItems adds stock for each item in a single transaction and returns updated rows.
// Optional Price on each item is recorded as stock expenditure for monthly totals.
func (h *IngredientHandler) applyRestockItems(restaurantID, createdBy string, items []RestockItem) ([]models.Ingredient, float64, error) {
	type qtyLine struct {
		id    string
		qty   float64
		price float64
		name  string
		unit  string
	}

	// Load ingredients first so we can convert entry units → inventory units.
	idSet := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.IngredientID != "" {
			idSet[item.IngredientID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var found []models.Ingredient
	if len(ids) > 0 {
		if err := h.db.Where("restaurant_id = ? AND id IN ?", restaurantID, ids).Find(&found).Error; err != nil {
			return nil, 0, err
		}
	}
	byID := make(map[string]models.Ingredient, len(found))
	for _, ing := range found {
		byID[ing.ID] = ing
	}

	ordered := make([]qtyLine, 0, len(items))
	indexByID := make(map[string]int, len(items))
	for _, item := range items {
		id := item.IngredientID
		if id == "" || item.Quantity <= 0 {
			continue
		}
		ing, ok := byID[id]
		if !ok {
			return nil, 0, fmt.Errorf("%w: %s", errIngredientNotFound, id)
		}
		entryUnit := strings.TrimSpace(item.Unit)
		if entryUnit == "" {
			entryUnit = ing.Unit
		}
		qty, err := units.Convert(item.Quantity, entryUnit, ing.Unit)
		if err != nil {
			return nil, 0, fmt.Errorf("ingredient %s: %w", id, err)
		}
		price := item.Price
		if price < 0 {
			price = 0
		}
		if idx, exists := indexByID[id]; exists {
			ordered[idx].qty += qty
			ordered[idx].price += price
			continue
		}
		indexByID[id] = len(ordered)
		ordered = append(ordered, qtyLine{
			id:    id,
			qty:   qty,
			price: price,
			name:  ing.Name,
			unit:  ing.Unit,
		})
	}
	if len(ordered) == 0 {
		return nil, 0, errNoValidRestockQuantities
	}

	updated := make([]models.Ingredient, 0, len(ordered))
	var expenditureAdded float64
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

			if line.price > 0 {
				ingID := line.id
				entry := models.StockExpenditure{
					RestaurantID:   restaurantID,
					IngredientID:   &ingID,
					IngredientName: line.name,
					Amount:         line.price,
					Quantity:       line.qty,
					Unit:           line.unit,
					Source:         "restock",
					CreatedBy:      createdBy,
				}
				if err := tx.Create(&entry).Error; err != nil {
					return err
				}
				expenditureAdded += line.price
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return updated, expenditureAdded, nil
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

	updated, expenditureAdded, err := h.applyRestockItems(restaurantID.(string), contextUserID(c), []RestockItem{
		{IngredientID: ingredientID, Quantity: req.Quantity, Unit: req.Unit, Price: req.Price},
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
		"message":            "Stock added successfully",
		"ingredient":         ingredient,
		"ingredients":        updated,
		"added":              req.Quantity,
		"updated":            len(updated),
		"expenditure_added":  expenditureAdded,
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

	updated, expenditureAdded, err := h.applyRestockItems(restaurantID.(string), contextUserID(c), req.Items)
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

	log.Printf("✅ Bulk restock: %d ingredients updated for restaurant %s (expenditure +%.2f)",
		len(updated), restaurantID, expenditureAdded)

	if globalHub != nil {
		for _, ingredient := range updated {
			BroadcastIngredientInventoryUpdate(globalHub, restaurantID.(string), ingredient)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Stock added successfully",
		"ingredients":       updated,
		"updated":           len(updated),
		"expenditure_added": expenditureAdded,
	})
}

// DeductIngredient removes quantity from current_stock (e.g. expired waste).
// Admin and manager only.
func (h *IngredientHandler) DeductIngredient(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req DeductIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var ingredient models.Ingredient
	if err := h.db.Where("id = ? AND restaurant_id = ?", req.IngredientID, restaurantID.(string)).
		First(&ingredient).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "ingredient not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	entryUnit := strings.TrimSpace(req.Unit)
	if entryUnit == "" {
		entryUnit = ingredient.Unit
	}
	qty, err := units.Convert(req.Quantity, entryUnit, ingredient.Unit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if qty <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity must be greater than 0"})
		return
	}
	if qty > ingredient.CurrentStock {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "insufficient stock",
			"current_stock":   ingredient.CurrentStock,
			"requested":       qty,
			"unit":            ingredient.Unit,
		})
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "expired"
	}

	if err := h.db.Model(&ingredient).
		Update("current_stock", gorm.Expr("current_stock - ?", qty)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.db.Where("id = ?", ingredient.ID).First(&ingredient).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Ingredient deducted (%s): %s -%.3f %s (now %.3f) by %s",
		reason, ingredient.Name, qty, ingredient.Unit, ingredient.CurrentStock, contextUserID(c))

	if globalHub != nil {
		BroadcastIngredientInventoryUpdate(globalHub, restaurantID.(string), ingredient)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Stock deducted successfully",
		"ingredient":  ingredient,
		"deducted":    qty,
		"unit":        ingredient.Unit,
		"reason":      reason,
	})
}

// GetMonthlyExpenditure returns total stock-refill spend for a calendar month (IST).
// Query: ?year=2026&month=7 — defaults to current IST month. Admin/manager only.
func (h *IngredientHandler) GetMonthlyExpenditure(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	loc := istLocation()
	now := time.Now().In(loc)
	year := now.Year()
	month := int(now.Month())

	if y := strings.TrimSpace(c.Query("year")); y != "" {
		parsed, err := strconv.Atoi(y)
		if err != nil || parsed < 2000 || parsed > 2100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid year"})
			return
		}
		year = parsed
	}
	if m := strings.TrimSpace(c.Query("month")); m != "" {
		parsed, err := strconv.Atoi(m)
		if err != nil || parsed < 1 || parsed > 12 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid month"})
			return
		}
		month = parsed
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)

	var total float64
	if err := h.db.Model(&models.StockExpenditure{}).
		Where("restaurant_id = ? AND created_at >= ? AND created_at < ?", restaurantID.(string), start.UTC(), end.UTC()).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var count int64
	_ = h.db.Model(&models.StockExpenditure{}).
		Where("restaurant_id = ? AND created_at >= ? AND created_at < ?", restaurantID.(string), start.UTC(), end.UTC()).
		Count(&count).Error

	c.JSON(http.StatusOK, gin.H{
		"year":             year,
		"month":            month,
		"total":            total,
		"entries":          count,
		"currency":         "INR",
		"period_start":     start.Format(time.RFC3339),
		"period_end":       end.Format(time.RFC3339),
	})
}
