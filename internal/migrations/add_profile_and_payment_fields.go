package migrations

import (
	"gorm.io/gorm"
)

// AddProfileAndPaymentFields adds new fields to support restaurant profile and payment completion
func AddProfileAndPaymentFields(db *gorm.DB) error {
	// Add ContactNumber and UPIQRCode to restaurants table
	if err := db.Exec(`
		ALTER TABLE restaurants 
		ADD COLUMN IF NOT EXISTS contact_number VARCHAR(50),
		ADD COLUMN IF NOT EXISTS upi_qr_code TEXT;
	`).Error; err != nil {
		return err
	}

	// Add AmountReceived and ChangeReturned to orders table
	if err := db.Exec(`
		ALTER TABLE orders 
		ADD COLUMN IF NOT EXISTS amount_received NUMERIC(10,2),
		ADD COLUMN IF NOT EXISTS change_returned NUMERIC(10,2);
	`).Error; err != nil {
		return err
	}

	// Add SubId to order_items table for batch tracking
	if err := db.Exec(`
		ALTER TABLE order_items 
		ADD COLUMN IF NOT EXISTS sub_id VARCHAR(100);
	`).Error; err != nil {
		return err
	}

	// Create index on sub_id for faster queries
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_order_items_sub_id ON order_items(sub_id);
	`).Error; err != nil {
		return err
	}

	return nil
}

// RollbackProfileAndPaymentFields removes the added fields
func RollbackProfileAndPaymentFields(db *gorm.DB) error {
	// Remove from restaurants
	if err := db.Exec(`
		ALTER TABLE restaurants 
		DROP COLUMN IF EXISTS contact_number,
		DROP COLUMN IF EXISTS upi_qr_code;
	`).Error; err != nil {
		return err
	}

	// Remove from orders
	if err := db.Exec(`
		ALTER TABLE orders 
		DROP COLUMN IF EXISTS amount_received,
		DROP COLUMN IF EXISTS change_returned;
	`).Error; err != nil {
		return err
	}

	// Remove from order_items
	if err := db.Exec(`
		DROP INDEX IF EXISTS idx_order_items_sub_id;
		ALTER TABLE order_items 
		DROP COLUMN IF EXISTS sub_id;
	`).Error; err != nil {
		return err
	}

	return nil
}
