package handlers

import (
	"errors"
	"strings"

	"restaurant-api/internal/models"
	"restaurant-api/internal/units"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RecipeIngredientInput is the request shape for one recipe line.
type RecipeIngredientInput struct {
	IngredientID string  `json:"ingredient_id"`
	Name         string  `json:"name"`
	Unit         string  `json:"unit"`
	QuantityUsed float64 `json:"quantity_used"`
}

func findIngredientByNameFamily(db *gorm.DB, restaurantID, name, unit string) (models.Ingredient, error) {
	name = strings.TrimSpace(name)
	canonical := units.CanonicalUnit(unit)
	family := units.FamilyOf(canonical)

	var existing models.Ingredient
	if family == units.FamilyOther {
		err := db.Where(
			"restaurant_id = ? AND LOWER(name) = ? AND LOWER(unit) = ?",
			restaurantID,
			strings.ToLower(name),
			strings.ToLower(canonical),
		).First(&existing).Error
		return existing, err
	}

	members := units.FamilyMemberUnits(family)
	err := db.Where(
		"restaurant_id = ? AND LOWER(name) = ? AND LOWER(unit) IN ?",
		restaurantID,
		strings.ToLower(name),
		lowerStrings(members),
	).Order("updated_at DESC").First(&existing).Error
	return existing, err
}

func lowerStrings(vals []string) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = strings.ToLower(v)
	}
	return out
}

func findOrCreateIngredient(db *gorm.DB, restaurantID, name, unit string) (models.Ingredient, error) {
	name = strings.TrimSpace(name)
	unit = strings.TrimSpace(unit)
	if name == "" || unit == "" {
		return models.Ingredient{}, errors.New("ingredient name and unit are required")
	}

	canonical := units.CanonicalUnit(unit)
	existing, err := findIngredientByNameFamily(db, restaurantID, name, unit)
	if err == nil {
		if units.NormalizeUnit(existing.Unit) != canonical {
			existing.Unit = canonical
			if saveErr := db.Save(&existing).Error; saveErr != nil {
				return models.Ingredient{}, saveErr
			}
			_ = syncRecipeDenormalizedNames(db, restaurantID, existing.ID, existing.Name, existing.Unit)
		}
		return existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return models.Ingredient{}, err
	}

	ingredient := models.Ingredient{
		ID:           uuid.New().String(),
		RestaurantID: restaurantID,
		Name:         name,
		Unit:         canonical,
	}
	if err := db.Create(&ingredient).Error; err != nil {
		return models.Ingredient{}, err
	}
	return ingredient, nil
}

func ingredientIdentityChanged(ing models.Ingredient, name, unit string) bool {
	if !strings.EqualFold(strings.TrimSpace(ing.Name), strings.TrimSpace(name)) {
		return true
	}
	if units.SameFamily(ing.Unit, unit) && units.FamilyOf(unit) != units.FamilyOther {
		return false
	}
	return units.NormalizeUnit(ing.Unit) != units.NormalizeUnit(unit)
}

// resolveIngredientForRecipeLine picks or creates the canonical inventory row for a recipe line.
func resolveIngredientForRecipeLine(db *gorm.DB, restaurantID string, input RecipeIngredientInput) (models.Ingredient, error) {
	name := strings.TrimSpace(input.Name)
	unit := strings.TrimSpace(input.Unit)

	if input.IngredientID != "" {
		var existing models.Ingredient
		if err := db.Where("id = ? AND restaurant_id = ?", input.IngredientID, restaurantID).
			First(&existing).Error; err != nil {
			if err != gorm.ErrRecordNotFound {
				return models.Ingredient{}, err
			}
			return findOrCreateIngredient(db, restaurantID, name, unit)
		}

		if name == "" {
			name = existing.Name
		}
		if unit == "" {
			unit = existing.Unit
		}

		if !ingredientIdentityChanged(existing, name, unit) {
			canonical := units.CanonicalUnit(existing.Unit)
			if units.NormalizeUnit(existing.Unit) != canonical {
				existing.Unit = canonical
				if err := db.Save(&existing).Error; err != nil {
					return models.Ingredient{}, err
				}
			}
			return existing, nil
		}

		var refCount int64
		if err := db.Model(&models.MenuItemIngredient{}).
			Where("restaurant_id = ? AND ingredient_id = ?", restaurantID, existing.ID).
			Count(&refCount).Error; err != nil {
			return models.Ingredient{}, err
		}

		if refCount == 0 {
			existing.Name = name
			existing.Unit = units.CanonicalUnit(unit)
			if err := db.Save(&existing).Error; err != nil {
				return models.Ingredient{}, err
			}
			return existing, nil
		}

		return findOrCreateIngredient(db, restaurantID, name, unit)
	}

	return findOrCreateIngredient(db, restaurantID, name, unit)
}

// recipeQuantityInInventoryUnit converts a recipe line qty into the inventory row unit.
func recipeQuantityInInventoryUnit(qty float64, fromUnit string, inventoryUnit string) (float64, error) {
	if fromUnit == "" {
		fromUnit = inventoryUnit
	}
	return units.Convert(qty, fromUnit, inventoryUnit)
}

func syncRecipeDenormalizedNames(db *gorm.DB, restaurantID, ingredientID, name, unit string) error {
	return db.Model(&models.MenuItemIngredient{}).
		Where("restaurant_id = ? AND ingredient_id = ?", restaurantID, ingredientID).
		Updates(map[string]interface{}{
			"name": strings.TrimSpace(name),
			"unit": strings.TrimSpace(unit),
		}).Error
}

func pruneOrphanIngredients(db *gorm.DB, restaurantID string) error {
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

func backfillMenuItemIngredientIDs(db *gorm.DB) error {
	var rows []models.MenuItemIngredient
	if err := db.Where("ingredient_id IS NULL OR ingredient_id = ''").Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		ingredient, err := findOrCreateIngredient(db, row.RestaurantID, row.Name, row.Unit)
		if err != nil {
			return err
		}
		qty := row.QuantityUsed
		if converted, convErr := recipeQuantityInInventoryUnit(row.QuantityUsed, row.Unit, ingredient.Unit); convErr == nil {
			qty = converted
		}
		if err := db.Model(&models.MenuItemIngredient{}).
			Where("id = ?", row.ID).
			Updates(map[string]interface{}{
				"ingredient_id": ingredient.ID,
				"name":          ingredient.Name,
				"unit":          ingredient.Unit,
				"quantity_used": qty,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncIngredientsFromRecipes links legacy rows and ensures inventory rows exist for all recipe ingredient_ids.
func syncIngredientsFromRecipes(db *gorm.DB, restaurantID string) error {
	if err := backfillMenuItemIngredientIDs(db); err != nil {
		return err
	}

	var recipes []models.MenuItemIngredient
	if err := db.Where("restaurant_id = ?", restaurantID).Find(&recipes).Error; err != nil {
		return err
	}

	seen := make(map[string]bool)
	for _, recipe := range recipes {
		if recipe.IngredientID == "" {
			continue
		}
		if seen[recipe.IngredientID] {
			continue
		}
		seen[recipe.IngredientID] = true

		var count int64
		if err := db.Model(&models.Ingredient{}).
			Where("id = ? AND restaurant_id = ?", recipe.IngredientID, restaurantID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		_, err := findOrCreateIngredient(db, restaurantID, recipe.Name, recipe.Unit)
		if err != nil {
			return err
		}
	}

	return pruneOrphanIngredients(db, restaurantID)
}
