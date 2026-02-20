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

	// Count orders before deletion
	var orderCount int64
	db.Model(&models.Order{}).Count(&orderCount)
	log.Printf("📊 Orders before deletion: %d", orderCount)

	var itemCount int64
	db.Model(&models.OrderItem{}).Count(&itemCount)
	log.Printf("📊 Order items before deletion: %d", itemCount)

	// First, clear the current_order_id from restaurant_tables to remove foreign key constraint
	updateResult := db.Table("restaurant_tables").
		Where("1=1").
		Update("current_order_id", nil)
	if updateResult.Error != nil {
		log.Fatalf("❌ Failed to clear current_order_id from tables: %v", updateResult.Error)
	}
	log.Printf("✅ Cleared current_order_id from %d table records", updateResult.RowsAffected)

	// Delete all order items first (foreign key constraint)
	result := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.OrderItem{})
	if result.Error != nil {
		log.Fatalf("❌ Failed to delete order items: %v", result.Error)
	}
	log.Printf("✅ Deleted %d order items", result.RowsAffected)

	// Delete all orders
	result = db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&models.Order{})
	if result.Error != nil {
		log.Fatalf("❌ Failed to delete orders: %v", result.Error)
	}
	log.Printf("✅ Deleted %d orders", result.RowsAffected)

	// Verify deletion
	db.Model(&models.Order{}).Count(&orderCount)
	log.Printf("📊 Orders after deletion: %d", orderCount)

	db.Model(&models.OrderItem{}).Count(&itemCount)
	log.Printf("📊 Order items after deletion: %d", itemCount)

	fmt.Println("\n✅ All orders and items have been deleted from the database!")
}
