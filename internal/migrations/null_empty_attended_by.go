package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

const nullEmptyAttendedByMigrationID = "null_empty_attended_by_user_id_v1"

// NullEmptyAttendedByUserID clears empty-string attended_by_user_id values so the
// FK constraint is not violated ("" is not a valid user id).
func NullEmptyAttendedByUserID(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS app_migrations (
			id TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		return fmt.Errorf("ensure app_migrations table: %w", err)
	}

	var alreadyApplied int64
	if err := db.Raw(
		`SELECT COUNT(1) FROM app_migrations WHERE id = ?`,
		nullEmptyAttendedByMigrationID,
	).Scan(&alreadyApplied).Error; err != nil {
		return fmt.Errorf("check null empty attended_by flag: %w", err)
	}
	if alreadyApplied > 0 {
		fmt.Println("⏭️  NullEmptyAttendedByUserID already applied; skipping")
		return nil
	}

	result := db.Exec(`
		UPDATE orders
		SET attended_by_user_id = NULL
		WHERE attended_by_user_id IS NOT NULL AND TRIM(attended_by_user_id) = ''
	`)
	if result.Error != nil {
		return fmt.Errorf("null empty attended_by_user_id: %w", result.Error)
	}

	if err := db.Exec(
		`INSERT INTO app_migrations (id) VALUES (?) ON CONFLICT (id) DO NOTHING`,
		nullEmptyAttendedByMigrationID,
	).Error; err != nil {
		return fmt.Errorf("record null empty attended_by migration: %w", err)
	}

	fmt.Printf("✅ NullEmptyAttendedByUserID complete (updated %d row(s))\n", result.RowsAffected)
	return nil
}
