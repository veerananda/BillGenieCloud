package services

import (
	"log"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

// MenuItemQuantity identifies how many units of a menu item affect ingredient stock.
type MenuItemQuantity struct {
	MenuItemID string
	Quantity   int
}

func menuItemQuantitiesFromCreateItems(items []CreateOrderItemRequest) []MenuItemQuantity {
	out := make([]MenuItemQuantity, 0, len(items))
	for _, item := range items {
		if item.Quantity < 1 {
			continue
		}
		out = append(out, MenuItemQuantity{
			MenuItemID: item.MenuItemID,
			Quantity:   item.Quantity,
		})
	}
	return out
}

func menuItemQuantitiesFromOrderItems(items []models.OrderItem) []MenuItemQuantity {
	out := make([]MenuItemQuantity, 0, len(items))
	for _, item := range items {
		if item.Quantity < 1 {
			continue
		}
		out = append(out, MenuItemQuantity{
			MenuItemID: item.MenuID,
			Quantity:   item.Quantity,
		})
	}
	return out
}

// DeductIngredientsForMenuItems subtracts recipe usage from ingredient current_stock.
func DeductIngredientsForMenuItems(tx *gorm.DB, restaurantID string, items []MenuItemQuantity) ([]models.Ingredient, error) {
	return adjustIngredientStockForMenuItems(tx, restaurantID, items, true)
}

// RestoreIngredientsForMenuItems adds recipe usage back to ingredient current_stock.
func RestoreIngredientsForMenuItems(tx *gorm.DB, restaurantID string, items []MenuItemQuantity) ([]models.Ingredient, error) {
	return adjustIngredientStockForMenuItems(tx, restaurantID, items, false)
}

func adjustIngredientStockForMenuItems(
	tx *gorm.DB,
	restaurantID string,
	items []MenuItemQuantity,
	deduct bool,
) ([]models.Ingredient, error) {
	if len(items) == 0 {
		return nil, nil
	}

	menuItemIDs := make([]string, 0, len(items))
	seenMenu := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.MenuItemID == "" || item.Quantity < 1 {
			continue
		}
		if _, ok := seenMenu[item.MenuItemID]; ok {
			continue
		}
		seenMenu[item.MenuItemID] = struct{}{}
		menuItemIDs = append(menuItemIDs, item.MenuItemID)
	}
	if len(menuItemIDs) == 0 {
		return nil, nil
	}

	var recipes []models.MenuItemIngredient
	if err := tx.Where("restaurant_id = ? AND menu_item_id IN ?", restaurantID, menuItemIDs).
		Find(&recipes).Error; err != nil {
		return nil, err
	}

	recipesByMenuItem := make(map[string][]models.MenuItemIngredient, len(menuItemIDs))
	for _, recipe := range recipes {
		recipesByMenuItem[recipe.MenuItemID] = append(recipesByMenuItem[recipe.MenuItemID], recipe)
	}

	stockDeltaByIngredient := make(map[string]float64)
	for _, item := range items {
		if item.MenuItemID == "" || item.Quantity < 1 {
			continue
		}
		for _, recipe := range recipesByMenuItem[item.MenuItemID] {
			if recipe.IngredientID == "" {
				continue
			}
			amount := recipe.QuantityUsed * float64(item.Quantity)
			if amount <= 0 {
				continue
			}
			if deduct {
				stockDeltaByIngredient[recipe.IngredientID] -= amount
			} else {
				stockDeltaByIngredient[recipe.IngredientID] += amount
			}
		}
	}

	updatedIDs := make(map[string]struct{})
	for ingredientID, delta := range stockDeltaByIngredient {
		if delta == 0 {
			continue
		}
		result := tx.Model(&models.Ingredient{}).
			Where("id = ? AND restaurant_id = ?", ingredientID, restaurantID).
			Update("current_stock", gorm.Expr("current_stock + ?", delta))
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected > 0 {
			updatedIDs[ingredientID] = struct{}{}
			action := "restored"
			if deduct {
				action = "deducted"
			}
			log.Printf("✅ Ingredient stock %s: %s (delta %.3f)", action, ingredientID, delta)
		}
	}

	if len(updatedIDs) == 0 {
		return nil, nil
	}

	ids := make([]string, 0, len(updatedIDs))
	for id := range updatedIDs {
		ids = append(ids, id)
	}

	var updated []models.Ingredient
	if err := tx.Where("id IN ? AND restaurant_id = ?", ids, restaurantID).Find(&updated).Error; err != nil {
		return nil, err
	}
	return updated, nil
}
