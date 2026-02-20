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

	// Update all tables to be vacant (green)
	result := db.Model(&models.RestaurantTable{}).
		Where("1=1").
		Update("is_occupied", false)

	if result.Error != nil {
		log.Fatalf("❌ Failed to update tables: %v", result.Error)
	}

	log.Printf("✅ Updated %d tables to vacant (green) status", result.RowsAffected)
	fmt.Printf("\n✅ All %d tables are now marked as vacant!\n", result.RowsAffected)
}
