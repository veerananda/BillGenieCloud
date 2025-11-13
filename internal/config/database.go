package config

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"restaurant-api/internal/models"
)

func InitializeDatabase(cfg *Config) *gorm.DB {
	// Build DSN
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
	)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
		return nil
	}

	log.Println("✅ Database connected successfully")
	return db
}

func MigrateDatabase(db *gorm.DB) {
	log.Println("🔄 Running database migrations...")

	err := db.AutoMigrate(
		&models.User{},
		&models.Restaurant{},
		&models.Order{},
		&models.OrderItem{},
		&models.MenuItem{},
		&models.Inventory{},
		&models.Transaction{},
		&models.AuditLog{},
	)

	if err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	log.Println("✅ Database migrations completed")
}

func CloseDatabase(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to close database: %v", err)
	}
	sqlDB.Close()
	log.Println("✅ Database connection closed")
}

// Seed initial data (optional)
func SeedDatabase(db *gorm.DB) {
	log.Println("🌱 Seeding database...")
	// Implement seeding logic here if needed
	log.Println("✅ Database seeding completed")
}
