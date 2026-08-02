package migrations

import "gorm.io/gorm"

// AddCanDeductInventory adds per-staff/chef permission for deducting expired stock.
func AddCanDeductInventory(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS can_deduct_inventory BOOLEAN NOT NULL DEFAULT false;
	`).Error
}
