package migrations

import (
	"gorm.io/gorm"
)

// AddIsApproved adds is_approved column for BillGenie staff approval gating.
// Existing restaurants are grandfathered as approved (DEFAULT true applies to
// pre-existing rows at column-creation time); the default is then flipped to
// false so every new registration starts out pending approval.
func AddIsApproved(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE restaurants ADD COLUMN IF NOT EXISTS is_approved BOOLEAN DEFAULT true;
		ALTER TABLE restaurants ALTER COLUMN is_approved SET DEFAULT false;
	`).Error
}
