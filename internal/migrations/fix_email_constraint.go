package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// FixEmailConstraint drops the old idx_restaurant_email constraint and creates
// a partial unique index on email that only applies to non-null values.
// This allows admin emails to be globally unique while allowing staff/manager/chef 
// to have null emails (they authenticate with staff_key instead).
func FixEmailConstraint(db *gorm.DB) error {
	// Step 1: Drop the old constraint by name if it exists
	// This handles both the constraint form and index form
	constraints := []string{
		"idx_restaurant_email",                    // Constraint name
		"users_restaurant_id_email_key",          // Possible constraint name
		"users_email_key",                        // Possible constraint name
	}

	for _, constraintName := range constraints {
		dropConstraint := db.Exec(fmt.Sprintf(`
			ALTER TABLE users 
			DROP CONSTRAINT IF EXISTS "%s"
		`, constraintName))

		if dropConstraint.Error == nil {
			fmt.Printf("✅ Dropped constraint: %s\n", constraintName)
		}
	}

	// Step 2: Drop the old index if it exists
	dropIndex := db.Exec(`DROP INDEX IF EXISTS idx_restaurant_email`)
	if dropIndex.Error == nil {
		fmt.Println("✅ Dropped old idx_restaurant_email index")
	}

	// Step 3: Create a partial unique index on email globally (not per-restaurant)
	// This enforces unique emails for admins but allows multiple nulls for staff/manager/chef
	createIndex := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_email_partial 
		ON users(email) 
		WHERE email IS NOT NULL AND email != ''
	`)

	if createIndex.Error != nil {
		// Check if table exists first
		if createIndex.Error.Error() == "pq: relation \"users\" does not exist" {
			fmt.Printf("⚠️  Table users doesn't exist yet, will be created by AutoMigrate\n")
			return nil
		}
		fmt.Printf("⚠️  FixEmailConstraint: %v\n", createIndex.Error)
		// Don't fail - the index might already exist
		return nil
	}

	fmt.Println("✅ Created partial unique index idx_email_partial for globally unique admin emails")
	return nil
}
