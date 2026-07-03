package migrations

import "gorm.io/gorm"

// AddCanRestockInventory adds per-staff permission for stock refill.
func AddCanRestockInventory(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS can_restock_inventory BOOLEAN NOT NULL DEFAULT false;
	`).Error
}
