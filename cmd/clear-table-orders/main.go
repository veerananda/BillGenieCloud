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
	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	log.Println("✅ Connected to database")

	// Clear all current_order_id references
	result := db.Model(&models.RestaurantTable{}).
		Where("1=1").
		Update("current_order_id", nil)

	if result.Error != nil {
		log.Fatalf("❌ Failed to clear current_order_id: %v", result.Error)
	}

	log.Printf("✅ Cleared current_order_id from %d tables", result.RowsAffected)
	fmt.Printf("\n✅ All %d tables now have no orders linked!\n", result.RowsAffected)
}
