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

	// Check status of specific tables
	var tables []models.RestaurantTable
	db.Where("name IN (?) OR id IN (?)", []string{"1C", "2A", "2D", "3A"}, []uint{1, 2, 3, 4}).Find(&tables)

	fmt.Println("\n📋 Current table status:")
	for _, table := range tables {
		fmt.Printf("  Table %s (ID: %d): is_occupied=%v, current_order_id=%v\n", 
			table.Name, table.ID, table.IsOccupied, table.CurrentOrderID)
	}

	// Check all tables
	var allTables []models.RestaurantTable
	db.Find(&allTables)
	fmt.Printf("\n📊 Total tables in database: %d\n", len(allTables))
	
	occupied := 0
	vacant := 0
	for _, t := range allTables {
		if t.IsOccupied {
			occupied++
			fmt.Printf("  ❌ %s (ID: %d) - OCCUPIED\n", t.Name, t.ID)
		} else {
			vacant++
		}
	}
	fmt.Printf("\nSummary: %d occupied, %d vacant\n", occupied, vacant)
}
