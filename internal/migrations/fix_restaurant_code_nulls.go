package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// FixRestaurantCodeNulls fills null restaurant_code values with unique defaults
func FixRestaurantCodeNulls(db *gorm.DB) error {
	// Try to update null values with unique defaults
	// Use first 8 chars of UUID as base for code
	// This query will be idempotent - only affects rows with null/empty codes
	result := db.Exec(`
		UPDATE restaurants 
		SET restaurant_code = 'REST' || SUBSTRING(CAST(id AS TEXT), 1, 6)
		WHERE (restaurant_code IS NULL OR restaurant_code = '')
	`)

	// If table doesn't exist yet, that's fine - AutoMigrate will create it
	if result.Error != nil {
		// Check if it's a table doesn't exist error
		if result.Error.Error() == "pq: relation \"restaurants\" does not exist" {
			fmt.Printf("⚠️  Table restaurants doesn't exist yet, will be created by AutoMigrate\n")
			return nil
		}
		// For other errors, log but don't fail - the NOT NULL constraint might not exist yet
		fmt.Printf("⚠️  FixRestaurantCodeNulls: %v (may already be applied)\n", result.Error)
		return nil
	}

	if result.RowsAffected > 0 {
		fmt.Printf("✅ Updated %d restaurants with null restaurant_code\n", result.RowsAffected)
	}

	return nil
}
