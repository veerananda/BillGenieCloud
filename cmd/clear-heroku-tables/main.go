package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"restaurant-api/internal/models"
)

func main() {
	// Use the RDS database URL that Heroku is using
	// You need to provide this as an environment variable
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL environment variable not set. Provide the Heroku/RDS database URL")
	}

	log.Printf("🔗 Connecting to database: %s", dbURL[:50]+"...")

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	log.Println("✅ Connected to database")

	// 1. Clear all current_order_id references first
	result := db.Model(&models.RestaurantTable{}).
		Where("1=1").
		Update("current_order_id", nil)

	if result.Error != nil {
		log.Fatalf("❌ Failed to clear current_order_id: %v", result.Error)
	}
	log.Printf("✅ Cleared current_order_id from %d tables", result.RowsAffected)

	// 2. Update all tables to be vacant (green)
	result2 := db.Model(&models.RestaurantTable{}).
		Where("1=1").
		Update("is_occupied", false)

	if result2.Error != nil {
		log.Fatalf("❌ Failed to update tables: %v", result2.Error)
	}

	log.Printf("✅ Updated %d tables to vacant (green) status", result2.RowsAffected)

	// 3. Verify the update
	var allTables []models.RestaurantTable
	db.Find(&allTables)
	
	fmt.Println("\n========================================")
	fmt.Println("✅ All tables updated! Current status:")
	fmt.Println("========================================")
	
	occupied := 0
	vacant := 0
	for _, t := range allTables {
		if t.IsOccupied {
			occupied++
			fmt.Printf("  ❌ %s - OCCUPIED\n", t.Name)
		} else {
			vacant++
			fmt.Printf("  ✅ %s - VACANT\n", t.Name)
		}
	}
	fmt.Printf("\n📊 Summary: %d occupied, %d vacant\n", occupied, vacant)
}
