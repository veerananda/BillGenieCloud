package config

import (
	"fmt"
	"log"
	"time"

	"restaurant-api/internal/migrations"
	"restaurant-api/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitializeDatabase(cfg *Config) *gorm.DB {
	// Build DSN with production SSL mode
	sslMode := "disable"
	if cfg.Environment == "production" {
		sslMode = "require" // Enforce SSL in production
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, sslMode,
	)

	// Connect to database
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
		return nil
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to configure connection pool: %v", err)
		return nil
	}

	// Set connection pool settings for production
	if cfg.Environment == "production" {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetMaxOpenConns(20)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
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
		&models.Ingredient{},
		&models.Transaction{},
		&models.AuditLog{},
		&models.RestaurantTable{},
		&models.RefreshToken{},
	)

	if err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	log.Println("✅ Database migrations completed")

	// Run custom migrations
	if err := migrations.ChangeTableNumberToString(db); err != nil {
		log.Printf("⚠️  Migration ChangeTableNumberToString skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ ChangeTableNumberToString migration completed")
	}
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
