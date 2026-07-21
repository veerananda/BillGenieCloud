package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// BackfillIngredientAlertQuantity adds alert_quantity (if missing) and sets
// existing rows to 25% of full_stock when alert_quantity is still 0 and full_stock > 0.
// Safe to re-run: only updates rows where alert_quantity is 0 and full_stock > 0.
func BackfillIngredientAlertQuantity(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE ingredients
		ADD COLUMN IF NOT EXISTS alert_quantity NUMERIC(10,2) NOT NULL DEFAULT 0
	`).Error; err != nil {
		return fmt.Errorf("add alert_quantity column: %w", err)
	}

	result := db.Exec(`
		UPDATE ingredients
		SET alert_quantity = ROUND((full_stock * 0.25)::numeric, 2)
		WHERE alert_quantity = 0 AND full_stock > 0
	`)
	if result.Error != nil {
		return fmt.Errorf("backfill alert_quantity: %w", result.Error)
	}
	fmt.Printf("✅ Backfilled alert_quantity on %d ingredient row(s)\n", result.RowsAffected)
	return nil
}
