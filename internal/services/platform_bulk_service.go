package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"restaurant-api/internal/models"
	"restaurant-api/internal/units"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func menuItemLookupKey(category, name string) string {
	return strings.ToLower(strings.TrimSpace(category)) + "\x00" + strings.ToLower(strings.TrimSpace(name))
}

func findMenuItemByCategoryAndName(db *gorm.DB, restaurantID, category, name string) (models.MenuItem, error) {
	var item models.MenuItem
	err := db.Where(
		"restaurant_id = ? AND LOWER(category) = ? AND LOWER(name) = ?",
		restaurantID,
		strings.ToLower(strings.TrimSpace(category)),
		strings.ToLower(strings.TrimSpace(name)),
	).First(&item).Error
	return item, err
}


type BulkRowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type BulkMenuUploadRow struct {
	Category           string  `json:"category"`
	Type               string  `json:"type"`
	Price              float64 `json:"price"`
	IsVeg              bool    `json:"is_veg"`
	IsAvailable        bool    `json:"is_available"`
	IsReadilyAvailable bool    `json:"is_readily_available"`
}

type BulkMenuUploadRequest struct {
	Reason string              `json:"reason"`
	Items  []BulkMenuUploadRow `json:"items"`
}

type BulkMenuUploadResult struct {
	Created int            `json:"created"`
	Updated int            `json:"updated"`
	Skipped int            `json:"skipped"`
	Errors  []BulkRowError   `json:"errors"`
	Items   []models.MenuItem `json:"items,omitempty"`
}

type BulkRecipeUploadRow struct {
	Category       string  `json:"category"`
	Type           string  `json:"type"`
	IngredientName string  `json:"ingredient_name"`
	Unit           string  `json:"unit"`
	Quantity       float64 `json:"quantity"`
}

type BulkRecipesUploadRequest struct {
	Reason string                `json:"reason"`
	Items  []BulkRecipeUploadRow `json:"items"`
}

type BulkRecipesUploadResult struct {
	MenusUpdated        int            `json:"menus_updated"`
	IngredientsCreated  int            `json:"ingredients_created"`
	RecipeLinesCreated  int            `json:"recipe_lines_created"`
	Errors              []BulkRowError   `json:"errors"`
}

func (s *PlatformOpsService) BulkUploadMenu(restaurantID string, req BulkMenuUploadRequest, actor string) (*BulkMenuUploadResult, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("items are required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	result := &BulkMenuUploadResult{Errors: []BulkRowError{}}
	savedItems := make([]models.MenuItem, 0, len(req.Items))

	for i, row := range req.Items {
		rowNum := i + 1
		category := strings.TrimSpace(row.Category)
		name := strings.TrimSpace(row.Type)
		if category == "" {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "category", Message: "category is required"})
			result.Skipped++
			continue
		}
		if name == "" {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "type", Message: "type (menu item name) is required"})
			result.Skipped++
			continue
		}
		price := row.Price
		if price <= 0 {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "price", Message: "price must be greater than 0"})
			result.Skipped++
			continue
		}

		existing, err := findMenuItemByCategoryAndName(s.db, restaurantID, category, name)

		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, err
		}

		if err == gorm.ErrRecordNotFound {
			item := models.MenuItem{
				ID:               uuid.New().String(),
				RestaurantID:     restaurantID,
				Name:             name,
				Category:         category,
				Price:            price,
				IsVeg:            row.IsVeg,
				IsAvailable:      row.IsAvailable,
				ReadilyAvailable: row.IsReadilyAvailable,
			}
			if err := s.db.Create(&item).Error; err != nil {
				result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Message: err.Error()})
				result.Skipped++
				continue
			}
			result.Created++
			savedItems = append(savedItems, item)
			continue
		}

		existing.Category = category
		existing.Price = price
		existing.IsVeg = row.IsVeg
		existing.IsAvailable = row.IsAvailable
		existing.ReadilyAvailable = row.IsReadilyAvailable
		if err := s.db.Save(&existing).Error; err != nil {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Message: err.Error()})
			result.Skipped++
			continue
		}
		result.Updated++
		savedItems = append(savedItems, existing)
	}

	oldSnapshot, _ := json.Marshal(map[string]interface{}{"menu_bulk": "before"})
	s.writePlatformAudit(restaurantID, actor, "platform_bulk_menu", reason, oldSnapshot, restaurant)
	result.Items = savedItems
	return result, nil
}

type recipeLineInput struct {
	rowNum         int
	category       string
	menuType       string
	ingredientName string
	unit           string
	quantity       float64
}

func (s *PlatformOpsService) BulkUploadRecipes(restaurantID string, req BulkRecipesUploadRequest, actor string) (*BulkRecipesUploadResult, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("items are required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	result := &BulkRecipesUploadResult{Errors: []BulkRowError{}}
	grouped := make(map[string][]recipeLineInput)
	menuOrder := make([]string, 0)

	for i, row := range req.Items {
		rowNum := i + 1
		category := strings.TrimSpace(row.Category)
		menuType := strings.TrimSpace(row.Type)
		ingredientName := strings.TrimSpace(row.IngredientName)
		unit := strings.TrimSpace(row.Unit)

		if category == "" {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "category", Message: "category is required"})
			continue
		}
		if menuType == "" {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "type", Message: "type (menu item name) is required"})
			continue
		}
		if ingredientName == "" {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "ingredient_name", Message: "ingredient name is required"})
			continue
		}
		if unit == "" {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "unit", Message: "unit is required"})
			continue
		}
		if row.Quantity <= 0 {
			result.Errors = append(result.Errors, BulkRowError{Row: rowNum, Field: "quantity", Message: "quantity must be greater than 0"})
			continue
		}

		key := menuItemLookupKey(category, menuType)
		if _, ok := grouped[key]; !ok {
			menuOrder = append(menuOrder, key)
			grouped[key] = []recipeLineInput{}
		}
		grouped[key] = append(grouped[key], recipeLineInput{
			rowNum:         rowNum,
			category:       category,
			menuType:       menuType,
			ingredientName: ingredientName,
			unit:           unit,
			quantity:       row.Quantity,
		})
	}

	ingredientsCreated := 0

	for _, menuKey := range menuOrder {
		lines := grouped[menuKey]
		if len(lines) == 0 {
			continue
		}

		displayCategory := lines[0].category
		displayType := lines[0].menuType

		menuItem, err := findMenuItemByCategoryAndName(s.db, restaurantID, displayCategory, displayType)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				for _, line := range lines {
					result.Errors = append(result.Errors, BulkRowError{
						Row:     line.rowNum,
						Field:   "type",
						Message: fmt.Sprintf("menu item %q in category %q not found — upload menu first", displayType, displayCategory),
					})
				}
				continue
			}
			return nil, err
		}

		tx := s.db.Begin()

		if err := tx.Where("restaurant_id = ? AND menu_item_id = ?", restaurantID, menuItem.ID).
			Delete(&models.MenuItemIngredient{}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		menuLinesCreated := 0
		menuHadError := false
		for _, line := range lines {
			ingredient, created, err := bulkFindOrCreateIngredient(tx, restaurantID, line.ingredientName, line.unit)
			if err != nil {
				tx.Rollback()
				result.Errors = append(result.Errors, BulkRowError{Row: line.rowNum, Message: err.Error()})
				menuHadError = true
				break
			}
			if created {
				ingredientsCreated++
			}

			qtyUsed, convErr := units.Convert(line.quantity, line.unit, ingredient.Unit)
			if convErr != nil {
				tx.Rollback()
				result.Errors = append(result.Errors, BulkRowError{Row: line.rowNum, Message: convErr.Error()})
				menuHadError = true
				break
			}

			row := models.MenuItemIngredient{
				ID:           uuid.New().String(),
				RestaurantID: restaurantID,
				MenuItemID:   menuItem.ID,
				IngredientID: ingredient.ID,
				Name:         ingredient.Name,
				Unit:         ingredient.Unit,
				QuantityUsed: qtyUsed,
			}
			if err := tx.Create(&row).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			menuLinesCreated++
		}

		if menuHadError || menuLinesCreated == 0 {
			continue
		}

		if err := tx.Commit().Error; err != nil {
			return nil, err
		}

		result.MenusUpdated++
		result.RecipeLinesCreated += menuLinesCreated
	}

	if err := bulkPruneOrphanIngredients(s.db, restaurantID); err != nil {
		return nil, err
	}

	oldSnapshot, _ := json.Marshal(map[string]interface{}{"recipes_bulk": "before"})
	s.writePlatformAudit(restaurantID, actor, "platform_bulk_recipes", reason, oldSnapshot, restaurant)
	result.IngredientsCreated = ingredientsCreated
	return result, nil
}

func bulkFindOrCreateIngredient(db *gorm.DB, restaurantID, name, unit string) (models.Ingredient, bool, error) {
	name = strings.TrimSpace(name)
	unit = strings.TrimSpace(unit)
	if name == "" || unit == "" {
		return models.Ingredient{}, false, errors.New("ingredient name and unit are required")
	}

	canonical := units.CanonicalUnit(unit)
	family := units.FamilyOf(canonical)

	var existing models.Ingredient
	var err error
	if family == units.FamilyOther {
		err = db.Where(
			"restaurant_id = ? AND LOWER(name) = ? AND LOWER(unit) = ?",
			restaurantID,
			strings.ToLower(name),
			strings.ToLower(canonical),
		).First(&existing).Error
	} else {
		members := units.FamilyMemberUnits(family)
		lowered := make([]string, len(members))
		for i, m := range members {
			lowered[i] = strings.ToLower(m)
		}
		err = db.Where(
			"restaurant_id = ? AND LOWER(name) = ? AND LOWER(unit) IN ?",
			restaurantID,
			strings.ToLower(name),
			lowered,
		).Order("updated_at DESC").First(&existing).Error
	}
	if err == nil {
		if units.NormalizeUnit(existing.Unit) != canonical {
			existing.Unit = canonical
			if saveErr := db.Save(&existing).Error; saveErr != nil {
				return models.Ingredient{}, false, saveErr
			}
		}
		return existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return models.Ingredient{}, false, err
	}

	ingredient := models.Ingredient{
		ID:           uuid.New().String(),
		RestaurantID: restaurantID,
		Name:         name,
		Unit:         canonical,
	}
	if err := db.Create(&ingredient).Error; err != nil {
		return models.Ingredient{}, false, err
	}
	return ingredient, true, nil
}

func bulkPruneOrphanIngredients(db *gorm.DB, restaurantID string) error {
	var usedIDs []string
	if err := db.Model(&models.MenuItemIngredient{}).
		Where("restaurant_id = ? AND ingredient_id <> ''", restaurantID).
		Distinct("ingredient_id").
		Pluck("ingredient_id", &usedIDs).Error; err != nil {
		return err
	}

	query := db.Where("restaurant_id = ?", restaurantID)
	if len(usedIDs) > 0 {
		query = query.Where("id NOT IN ?", usedIDs)
	}
	return query.Delete(&models.Ingredient{}).Error
}
