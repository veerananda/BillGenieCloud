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
	updatedIDs := make(map[string]struct{})

	for _, item := range items {
		var recipes []models.MenuItemIngredient
		if err := tx.Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, item.MenuItemID).
			Find(&recipes).Error; err != nil {
			return nil, err
		}

		for _, recipe := range recipes {
			if recipe.IngredientID == "" {
				continue
			}
			amount := recipe.QuantityUsed * float64(item.Quantity)
			if amount <= 0 {
				continue
			}

			expr := gorm.Expr("current_stock + ?", amount)
			if deduct {
				expr = gorm.Expr("current_stock - ?", amount)
			}

			result := tx.Model(&models.Ingredient{}).
				Where("id = ? AND restaurant_id = ?", recipe.IngredientID, restaurantID).
				Update("current_stock", expr)
			if result.Error != nil {
				return nil, result.Error
			}
			if result.RowsAffected > 0 {
				updatedIDs[recipe.IngredientID] = struct{}{}
				action := "restored"
				if deduct {
					action = "deducted"
				}
				log.Printf("✅ Ingredient stock %s: %s (%.3f %s) for menu item %s",
					action, recipe.IngredientID, amount, recipe.Unit, item.MenuItemID)
			}
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
