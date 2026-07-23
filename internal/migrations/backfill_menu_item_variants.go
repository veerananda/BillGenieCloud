package migrations

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const backfillMenuItemVariantsMigrationID = "backfill_menu_item_variants_v1"

// BackfillMenuItemVariants creates a default "Regular" variant for every menu item
// that does not yet have variants (keeps existing price as the variant price).
func BackfillMenuItemVariants(db *gorm.DB) error {
	var already int64
	if err := db.Table("schema_migrations").
		Where("id = ?", backfillMenuItemVariantsMigrationID).
		Count(&already).Error; err != nil {
		// table may not exist yet — create via AutoMigrate elsewhere; continue
		_ = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (id text PRIMARY KEY)`).Error
	} else if already > 0 {
		return nil
	}

	type row struct {
		ID           string
		RestaurantID string
		Price        float64
	}
	var items []row
	if err := db.Raw(`
		SELECT m.id, m.restaurant_id, m.price
		FROM menu_items m
		WHERE NOT EXISTS (
			SELECT 1 FROM menu_item_variants v WHERE v.menu_item_id = m.id
		)
	`).Scan(&items).Error; err != nil {
		return fmt.Errorf("list menu items without variants: %w", err)
	}

	for _, item := range items {
		id := uuid.New().String()
		if err := db.Exec(`
			INSERT INTO menu_item_variants
				(id, restaurant_id, menu_item_id, label, price, recipe_scale, is_default, is_available, sort_order, created_at, updated_at)
			VALUES
				(?, ?, ?, 'Regular', ?, 1, true, true, 0, NOW(), NOW())
		`, id, item.RestaurantID, item.ID, item.Price).Error; err != nil {
			return fmt.Errorf("insert default variant for %s: %w", item.ID, err)
		}
	}

	_ = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (id text PRIMARY KEY)`).Error
	if err := db.Exec(`INSERT INTO schema_migrations (id) VALUES (?) ON CONFLICT DO NOTHING`, backfillMenuItemVariantsMigrationID).Error; err != nil {
		return fmt.Errorf("record backfill variants migration: %w", err)
	}
	return nil
}
