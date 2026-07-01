package migrations

import (
	"gorm.io/gorm"
)

// AddUpiID adds upi_id column for dynamic UPI payment links.
func AddUpiID(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE restaurants
		ADD COLUMN IF NOT EXISTS upi_id VARCHAR(100);
	`).Error
}
