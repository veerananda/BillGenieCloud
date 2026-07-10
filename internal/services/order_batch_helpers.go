package services

import (
	"errors"
	"strings"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

func uniqueMenuItemIDs(items []CreateOrderItemRequest) []string {
	ids := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.MenuItemID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func loadMenuItemsMap(db *gorm.DB, restaurantID string, menuItemIDs []string) (map[string]models.MenuItem, error) {
	if len(menuItemIDs) == 0 {
		return map[string]models.MenuItem{}, nil
	}

	var menuItems []models.MenuItem
	if err := db.Where("restaurant_id = ? AND id IN ?", restaurantID, menuItemIDs).Find(&menuItems).Error; err != nil {
		return nil, err
	}

	menuByID := make(map[string]models.MenuItem, len(menuItems))
	for _, item := range menuItems {
		menuByID[item.ID] = item
	}

	for _, id := range menuItemIDs {
		if _, ok := menuByID[id]; !ok {
			return nil, errors.New("menu item not found")
		}
	}

	return menuByID, nil
}

func loadInventoryByMenuMap(tx *gorm.DB, restaurantID string, menuItemIDs []string) (map[string]models.Inventory, error) {
	if len(menuItemIDs) == 0 {
		return map[string]models.Inventory{}, nil
	}

	var inventories []models.Inventory
	if err := tx.Where("restaurant_id = ? AND menu_item_id IN ?", restaurantID, menuItemIDs).
		Find(&inventories).Error; err != nil {
		return nil, err
	}

	invByMenuID := make(map[string]models.Inventory, len(inventories))
	for _, inv := range inventories {
		invByMenuID[inv.MenuItemID] = inv
	}
	return invByMenuID, nil
}
