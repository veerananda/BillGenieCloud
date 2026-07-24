package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/migrations"
	"restaurant-api/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func buildDatabaseDSN(cfg *Config) string {
	// Prefer DATABASE_URL when explicitly set (Fly.io, DO Managed Postgres, Upstash-style hosts)
	if url := strings.TrimSpace(os.Getenv("DATABASE_URL")); url != "" {
		if cfg.Environment == "production" && !strings.Contains(url, "sslmode=") {
			sep := "?"
			if strings.Contains(url, "?") {
				sep = "&"
			}
			return url + sep + "sslmode=require"
		}
		return url
	}

	sslMode := "disable"
	if cfg.Environment == "production" {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, sslMode,
	)
}

func InitializeDatabase(cfg *Config) *gorm.DB {
	dsn := buildDatabaseDSN(cfg)

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

	// Fix any null staff_key values BEFORE AutoMigrate adds NOT NULL constraint
	if err := migrations.FixStaffKeyNulls(db); err != nil {
		log.Printf("⚠️  Migration FixStaffKeyNulls skipped or failed (may already be fixed): %v", err)
	} else {
		log.Println("✅ FixStaffKeyNulls migration completed")
	}

	// Fix any null restaurant codes BEFORE AutoMigrate adds NOT NULL constraint
	if err := migrations.FixRestaurantCodeNulls(db); err != nil {
		log.Printf("⚠️  Migration FixRestaurantCodeNulls skipped or failed (may already be fixed): %v", err)
	} else {
		log.Println("✅ FixRestaurantCodeNulls migration completed")
	}

	// Fix email constraint: drop old global unique constraint and create partial per-restaurant constraint
	if err := migrations.FixEmailConstraint(db); err != nil {
		log.Printf("⚠️  Migration FixEmailConstraint skipped or failed (may already be fixed): %v", err)
	} else {
		log.Println("✅ FixEmailConstraint migration completed")
	}

	// Add is_approved BEFORE AutoMigrate so existing restaurants are grandfathered
	// as approved instead of being locked out by the new default-false column.
	if err := migrations.AddIsApproved(db); err != nil {
		log.Printf("⚠️  Migration AddIsApproved skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ AddIsApproved migration completed")
	}

	// Drop misnamed UNIQUE constraints on assistance_token before AutoMigrate.
	// Otherwise GORM tries DROP CONSTRAINT uni_restaurant_tables_assistance_token
	// and fatals when that name does not exist (SQLSTATE 42704).
	if err := migrations.FixAssistanceTokenUniqueConstraint(db); err != nil {
		log.Fatalf("❌ FixAssistanceTokenUniqueConstraint failed: %v", err)
	}
	log.Println("✅ FixAssistanceTokenUniqueConstraint migration completed")

	// Now run AutoMigrate on all models
	err := db.AutoMigrate(
		&models.User{},
		&models.Restaurant{},
		&models.Order{},
		&models.OrderItem{},
		&models.MenuItem{},
		&models.MenuItemVariant{},
		&models.Inventory{},
		&models.Ingredient{},
		&models.StockExpenditure{},
		&models.Expense{},
		&models.MenuItemIngredient{},
		&models.Transaction{},
		&models.AuditLog{},
		&models.RestaurantTable{},
		&models.RefreshToken{},
		&models.UserSession{},
		&models.PasswordReset{},
		&models.LoginRecoveryOTP{},
		&models.EmailVerification{},
		&models.SubscriptionRenewal{},
		&models.TrialEligibility{},
		&models.SupportIssue{},
	)

	if err != nil {
		log.Fatalf("❌ Failed to migrate database: %v", err)
	}

	log.Println("✅ Database migrations completed")

	if err := migrations.BackfillIngredientAlertQuantity(db); err != nil {
		log.Printf("⚠️  Migration BackfillIngredientAlertQuantity skipped or failed: %v", err)
	} else {
		log.Println("✅ BackfillIngredientAlertQuantity migration completed")
	}

	if err := migrations.BackfillMenuItemVariants(db); err != nil {
		log.Printf("⚠️  Migration BackfillMenuItemVariants skipped or failed: %v", err)
	} else {
		log.Println("✅ BackfillMenuItemVariants migration completed")
	}

	if err := migrations.NullEmptyAttendedByUserID(db); err != nil {
		log.Printf("⚠️  Migration NullEmptyAttendedByUserID skipped or failed: %v", err)
	} else {
		log.Println("✅ NullEmptyAttendedByUserID migration completed")
	}

	// Enforce NOT NULL constraint on restaurant_code column after fixing nulls
	if err := migrations.EnforceRestaurantCodeNotNull(db); err != nil {
		log.Printf("⚠️  Migration EnforceRestaurantCodeNotNull failed: %v", err)
	} else {
		log.Println("✅ EnforceRestaurantCodeNotNull migration completed")
	}

	// Run remaining custom migrations

	if err := migrations.ChangeTableNumberToString(db); err != nil {
		log.Printf("⚠️  Migration ChangeTableNumberToString skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ ChangeTableNumberToString migration completed")
	}

	// Fix user email uniqueness to be per-restaurant instead of global
	if err := migrations.FixUserEmailUniqueness(db); err != nil {
		log.Printf("⚠️  Migration FixUserEmailUniqueness skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ FixUserEmailUniqueness migration completed")
	}

	if err := migrations.AddUpiID(db); err != nil {
		log.Printf("⚠️  Migration AddUpiID skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ AddUpiID migration completed")
	}

	if err := migrations.AddMenuItemIngredientID(db); err != nil {
		log.Printf("⚠️  Migration AddMenuItemIngredientID skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ AddMenuItemIngredientID migration completed")
	}

	if err := migrations.AddCanRestockInventory(db); err != nil {
		log.Printf("⚠️  Migration AddCanRestockInventory skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ AddCanRestockInventory migration completed")
	}

	if err := migrations.AddMenuManagementAccess(db); err != nil {
		log.Printf("⚠️  Migration AddMenuManagementAccess skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ AddMenuManagementAccess migration completed")
	}

	if err := migrations.AddPerformanceIndexes(db); err != nil {
		log.Printf("⚠️  Migration AddPerformanceIndexes skipped or failed (may already be applied): %v", err)
	} else {
		log.Println("✅ AddPerformanceIndexes migration completed")
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
