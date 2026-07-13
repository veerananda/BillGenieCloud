package migrations

import "gorm.io/gorm"

// AddMenuManagementAccess adds per-manager permission for full menu management.
func AddMenuManagementAccess(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE users
		ADD COLUMN IF NOT EXISTS menu_management_access BOOLEAN NOT NULL DEFAULT false;
	`).Error
}
