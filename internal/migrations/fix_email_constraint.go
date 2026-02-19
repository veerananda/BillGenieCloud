package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// FixEmailConstraint drops the old global unique constraint on email and creates
// a per-restaurant partial unique index that only applies to non-null emails.
// This allows multiple staff members to have null emails while enforcing admin email uniqueness per restaurant.
func FixEmailConstraint(db *gorm.DB) error {
	// Step 1: Drop the old idx_restaurant_email unique constraint if it exists
	dropConstraint := db.Exec(`
		ALTER TABLE users 
		DROP CONSTRAINT IF EXISTS idx_restaurant_email
	`)

	if dropConstraint.Error != nil {
		// It's okay if constraint doesn't exist
		fmt.Printf("⚠️  Could not drop idx_restaurant_email (may not exist): %v\n", dropConstraint.Error)
	} else {
		fmt.Println("✅ Dropped old idx_restaurant_email constraint")
	}

	// Step 2: Create a partial unique index that only applies to non-null emails
	// This enforces unique emails per restaurant (for admins) but allows multiple nulls (for staff)
	createIndex := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_email_partial 
		ON users(restaurant_id, email) 
		WHERE email IS NOT NULL AND email != ''
	`)

	if createIndex.Error != nil {
		// Check if table exists first
		if createIndex.Error.Error() == "pq: relation \"users\" does not exist" {
			fmt.Printf("⚠️  Table users doesn't exist yet, will be created by AutoMigrate\n")
			return nil
		}
		fmt.Printf("⚠️  FixEmailConstraint: %v (may already be applied)\n", createIndex.Error)
		return nil
	}

	fmt.Println("✅ Created partial unique index idx_restaurant_email_partial for non-null emails")
	return nil
}
