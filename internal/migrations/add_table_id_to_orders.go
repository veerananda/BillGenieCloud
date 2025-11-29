package migrations

import (
	"gorm.io/gorm"
)

// AddTableIDToOrders adds table_id field to link orders to dine-in tables
func AddTableIDToOrders(db *gorm.DB) error {
	// Add table_id to orders table
	if err := db.Exec(`
		ALTER TABLE orders 
		ADD COLUMN IF NOT EXISTS table_id VARCHAR(36);
	`).Error; err != nil {
		return err
	}

	// Create index on table_id for faster queries
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_orders_table_id ON orders(table_id);
	`).Error; err != nil {
		return err
	}

	return nil
}

// RollbackTableIDFromOrders removes the table_id field
func RollbackTableIDFromOrders(db *gorm.DB) error {
	// Remove index first
	if err := db.Exec(`
		DROP INDEX IF EXISTS idx_orders_table_id;
	`).Error; err != nil {
		return err
	}

	// Remove column from orders
	if err := db.Exec(`
		ALTER TABLE orders 
		DROP COLUMN IF EXISTS table_id;
	`).Error; err != nil {
		return err
	}

	return nil
}
