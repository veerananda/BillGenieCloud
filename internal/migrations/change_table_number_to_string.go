package migrations

import (
	"gorm.io/gorm"
)

// ChangeTableNumberToString changes table_number column from int to varchar
func ChangeTableNumberToString(db *gorm.DB) error {
	// Change table_number column type from integer to varchar
	if err := db.Exec(`
		ALTER TABLE orders 
		ALTER COLUMN table_number TYPE VARCHAR(50);
	`).Error; err != nil {
		return err
	}

	return nil
}

// RollbackTableNumberToInt reverts table_number column back to int
func RollbackTableNumberToInt(db *gorm.DB) error {
	// Change table_number column type back from varchar to integer
	if err := db.Exec(`
		ALTER TABLE orders 
		ALTER COLUMN table_number TYPE INTEGER;
	`).Error; err != nil {
		return err
	}

	return nil
}
