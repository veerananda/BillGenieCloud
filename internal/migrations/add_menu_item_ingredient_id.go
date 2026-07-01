package migrations

import "gorm.io/gorm"

// AddMenuItemIngredientID links recipe rows to canonical inventory ingredients.
func AddMenuItemIngredientID(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE menu_item_ingredients
		ADD COLUMN IF NOT EXISTS ingredient_id VARCHAR(36);
		CREATE INDEX IF NOT EXISTS idx_menu_item_ingredients_ingredient_id ON menu_item_ingredients(ingredient_id);
	`).Error
}
