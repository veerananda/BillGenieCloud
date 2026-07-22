package migrations

import (
	"fmt"
	"strings"

	"restaurant-api/internal/models"
	"restaurant-api/internal/units"

	"gorm.io/gorm"
)

const canonicalizeIngredientUnitsMigrationID = "canonicalize_ingredient_units_v1"

// CanonicalizeIngredientUnits converts grams/ml inventory to kg/liters once,
// rescales linked recipe quantities, and merges duplicate name+family rows.
func CanonicalizeIngredientUnits(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS app_migrations (
			id TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		return fmt.Errorf("ensure app_migrations table: %w", err)
	}

	var alreadyApplied int64
	if err := db.Raw(
		`SELECT COUNT(1) FROM app_migrations WHERE id = ?`,
		canonicalizeIngredientUnitsMigrationID,
	).Scan(&alreadyApplied).Error; err != nil {
		return fmt.Errorf("check canonicalize units flag: %w", err)
	}
	if alreadyApplied > 0 {
		fmt.Println("⏭️  CanonicalizeIngredientUnits already applied; skipping")
		return nil
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var ingredients []models.Ingredient
		if err := tx.Find(&ingredients).Error; err != nil {
			return err
		}

		updatedIngredients := 0
		for _, ing := range ingredients {
			fromUnit := ing.Unit
			canonical := units.CanonicalUnit(fromUnit)
			if units.NormalizeUnit(fromUnit) == canonical {
				continue
			}
			if units.FamilyOf(fromUnit) == units.FamilyOther {
				// Normalize spelling only when possible.
				if n := units.NormalizeUnit(fromUnit); n != "" && n != fromUnit {
					if err := tx.Model(&ing).Update("unit", n).Error; err != nil {
						return err
					}
					updatedIngredients++
				}
				continue
			}

			current, err := units.Convert(ing.CurrentStock, fromUnit, canonical)
			if err != nil {
				return fmt.Errorf("ingredient %s stock: %w", ing.ID, err)
			}
			full, err := units.Convert(ing.FullStock, fromUnit, canonical)
			if err != nil {
				return fmt.Errorf("ingredient %s full: %w", ing.ID, err)
			}
			alert, err := units.Convert(ing.AlertQuantity, fromUnit, canonical)
			if err != nil {
				return fmt.Errorf("ingredient %s alert: %w", ing.ID, err)
			}

			if err := tx.Model(&ing).Updates(map[string]interface{}{
				"unit":           canonical,
				"current_stock":  current,
				"full_stock":     full,
				"alert_quantity": alert,
			}).Error; err != nil {
				return err
			}
			updatedIngredients++
		}

		var recipes []models.MenuItemIngredient
		if err := tx.Find(&recipes).Error; err != nil {
			return err
		}
		updatedRecipes := 0
		for _, row := range recipes {
			fromUnit := row.Unit
			canonical := units.CanonicalUnit(fromUnit)
			if units.FamilyOf(fromUnit) == units.FamilyOther {
				if n := units.NormalizeUnit(fromUnit); n != "" && n != fromUnit {
					if err := tx.Model(&row).Update("unit", n).Error; err != nil {
						return err
					}
					updatedRecipes++
				}
				continue
			}
			if units.NormalizeUnit(fromUnit) == canonical {
				continue
			}
			qty, err := units.Convert(row.QuantityUsed, fromUnit, canonical)
			if err != nil {
				return fmt.Errorf("recipe %s qty: %w", row.ID, err)
			}
			if err := tx.Model(&row).Updates(map[string]interface{}{
				"unit":          canonical,
				"quantity_used": qty,
			}).Error; err != nil {
				return err
			}
			updatedRecipes++
		}

		if err := mergeDuplicateCanonicalIngredients(tx); err != nil {
			return err
		}

		fmt.Printf("✅ CanonicalizeIngredientUnits converted %d ingredients, %d recipe lines\n", updatedIngredients, updatedRecipes)
		return nil
	})
	if err != nil {
		return err
	}

	if err := db.Exec(
		`INSERT INTO app_migrations (id) VALUES (?) ON CONFLICT (id) DO NOTHING`,
		canonicalizeIngredientUnitsMigrationID,
	).Error; err != nil {
		return fmt.Errorf("record canonicalize units migration: %w", err)
	}
	return nil
}

func mergeDuplicateCanonicalIngredients(tx *gorm.DB) error {
	var ingredients []models.Ingredient
	if err := tx.Order("updated_at ASC").Find(&ingredients).Error; err != nil {
		return err
	}

	type key struct {
		restaurantID string
		name         string
		family       units.Family
		unit         string
	}
	keepers := map[key]*models.Ingredient{}

	for i := range ingredients {
		ing := &ingredients[i]
		family := units.FamilyOf(ing.Unit)
		k := key{
			restaurantID: ing.RestaurantID,
			name:         strings.ToLower(strings.TrimSpace(ing.Name)),
			family:       family,
			unit:         units.NormalizeUnit(ing.Unit),
		}
		if family == units.FamilyOther {
			// Only merge exact normalized unit duplicates for custom units.
		} else {
			k.unit = units.CanonicalUnit(ing.Unit)
		}

		if existing, ok := keepers[k]; ok {
			existing.CurrentStock += ing.CurrentStock
			if ing.FullStock > existing.FullStock {
				existing.FullStock = ing.FullStock
			}
			if ing.AlertQuantity > existing.AlertQuantity {
				existing.AlertQuantity = ing.AlertQuantity
			}
			if err := tx.Model(existing).Updates(map[string]interface{}{
				"current_stock":  existing.CurrentStock,
				"full_stock":     existing.FullStock,
				"alert_quantity": existing.AlertQuantity,
				"unit":           k.unit,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.MenuItemIngredient{}).
				Where("ingredient_id = ?", ing.ID).
				Updates(map[string]interface{}{
					"ingredient_id": existing.ID,
					"name":          existing.Name,
					"unit":          existing.Unit,
				}).Error; err != nil {
				return err
			}
			if err := tx.Delete(ing).Error; err != nil {
				return err
			}
			continue
		}
		keepers[k] = ing
	}
	return nil
}
