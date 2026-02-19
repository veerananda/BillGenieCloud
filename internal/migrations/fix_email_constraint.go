package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// FixEmailConstraint removes the problematic email constraint from the database.
// The constraint prevents multiple staff from having NULL emails.
// Instead, we'll rely on the code-level validation and staff_key as the unique identifier.
func FixEmailConstraint(db *gorm.DB) error {
	// Step 1: Query the database to find ALL constraints on the users table
	type ConstraintInfo struct {
		ConstraintName string
		ConstraintType string
	}

	var constraints []ConstraintInfo
	err := db.Raw(`
		SELECT constraint_name, constraint_type
		FROM information_schema.table_constraints
		WHERE table_name = 'users'
		AND table_schema = 'public'
	`).Scan(&constraints).Error

	if err != nil {
		fmt.Printf("⚠️  Could not query constraints: %v\n", err)
		// Continue anyway - table might not exist yet
	}

	// Step 2: Drop any constraint that looks like an email constraint
	emailConstraintNames := []string{}
	for _, c := range constraints {
		if c.ConstraintName == "idx_restaurant_email" || 
		   c.ConstraintName == "users_restaurant_id_email_key" ||
		   c.ConstraintName == "users_email_key" {
			emailConstraintNames = append(emailConstraintNames, c.ConstraintName)
		}
	}

	fmt.Printf("Found email constraints: %v\n", emailConstraintNames)

	for _, constraintName := range emailConstraintNames {
		dropSQL := fmt.Sprintf(`ALTER TABLE users DROP CONSTRAINT IF EXISTS "%s" CASCADE`, constraintName)
		if result := db.Exec(dropSQL); result.Error != nil {
			fmt.Printf("⚠️  Could not drop constraint %s: %v\n", constraintName, result.Error)
		} else {
			fmt.Printf("✅ Dropped constraint: %s\n", constraintName)
		}
	}

	// Step 3: Also drop any index with email in the name (except the ones we want to keep)
	indexes := []string{"idx_restaurant_email", "idx_email_partial"}
	for _, indexName := range indexes {
		db.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS "%s" CASCADE`, indexName))
	}

	// Step 4: Create a partial unique index that allows NULL values
	// NULL values don't violate uniqueness in PostgreSQL per SQL standard
	createIndex := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_email_partial 
		ON users(email) 
		WHERE email IS NOT NULL AND email != ''
	`)

	if createIndex.Error != nil {
		fmt.Printf("⚠️  Could not create partial index: %v\n", createIndex.Error)
	} else {
		fmt.Println("✅ Created partial unique index idx_email_partial for admin emails")
	}

	fmt.Println("✅ Email constraint migration completed")
	return nil
}
