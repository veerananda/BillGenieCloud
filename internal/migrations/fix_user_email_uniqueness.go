package migrations

import (
	"gorm.io/gorm"
)

// FixUserEmailUniqueness changes email uniqueness from global to per-restaurant
// This allows the same email to be used across different restaurants
func FixUserEmailUniqueness(db *gorm.DB) error {
	// Drop the old global unique constraint on email
	if err := db.Exec(`
		ALTER TABLE users 
		DROP CONSTRAINT IF EXISTS users_email_key;
	`).Error; err != nil {
		return err
	}

	// Create a composite unique index on (restaurant_id, email)
	// This ensures email is unique only within each restaurant
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_email 
		ON users(restaurant_id, email);
	`).Error; err != nil {
		return err
	}

	return nil
}

// RollbackUserEmailUniqueness reverts the email uniqueness change
func RollbackUserEmailUniqueness(db *gorm.DB) error {
	// Drop the composite unique index
	if err := db.Exec(`
		DROP INDEX IF EXISTS idx_restaurant_email;
	`).Error; err != nil {
		return err
	}

	// Recreate the global unique constraint
	// Note: This will fail if there are duplicate emails across restaurants
	if err := db.Exec(`
		ALTER TABLE users 
		ADD CONSTRAINT users_email_key UNIQUE (email);
	`).Error; err != nil {
		return err
	}

	return nil
}
