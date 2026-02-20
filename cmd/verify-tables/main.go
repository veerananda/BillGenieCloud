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
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	// Get all tables with their current order info
	var tables []models.RestaurantTable
	db.Preload("CurrentOrder").Order("name ASC").Find(&tables)

	fmt.Println("\n✅ Table Data from Database (what API should return):")
	fmt.Println("========================================")
	for _, table := range tables {
		status := "VACANT (Green)"
		if table.IsOccupied {
			status = "OCCUPIED (Red)"
		}
		fmt.Printf("%s (ID: %d): %s\n", table.Name, table.ID, status)
		if table.CurrentOrderID != nil {
			fmt.Printf("  └─ Current Order ID: %d\n", *table.CurrentOrderID)
		}
	}
	fmt.Println("========================================")
	fmt.Println("All tables should be VACANT according to database!")
}
