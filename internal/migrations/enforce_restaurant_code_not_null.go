package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// EnforceRestaurantCodeNotNull adds NOT NULL constraint to restaurant_code column after fixing nulls
func EnforceRestaurantCodeNotNull(db *gorm.DB) error {
	// Try to add the NOT NULL constraint
	result := db.Exec(`
		ALTER TABLE restaurants 
		ALTER COLUMN restaurant_code SET NOT NULL
	`)

	if result.Error != nil {
		// Check if it's already enforced or column doesn't exist
		errMsg := result.Error.Error()
		if errMsg == "pq: relation \"restaurants\" does not exist" {
			fmt.Printf("⚠️  Table restaurants doesn't exist yet\n")
			return nil
		}
		// Column might already have NOT NULL constraint
		if errMsg == "pq: column \"restaurant_code\" of relation \"restaurants\" is already NOT NULL" {
			fmt.Printf("✅ restaurant_code already has NOT NULL constraint\n")
			return nil
		}
		fmt.Printf("⚠️  EnforceRestaurantCodeNotNull: %v\n", result.Error)
		return nil
	}

	fmt.Printf("✅ Added NOT NULL constraint to restaurant_code column\n")
	return nil
}
