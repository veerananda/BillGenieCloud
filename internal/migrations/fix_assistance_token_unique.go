package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// FixAssistanceTokenUniqueConstraint prevents AutoMigrate from crashing when
// GORM's MigrateColumnUnique tries to DROP CONSTRAINT
// uni_restaurant_tables_assistance_token.
//
// That happens when Postgres has a UNIQUE constraint on assistance_token under
// a different name (e.g. restaurant_tables_assistance_token_key) while the
// model only declares uniqueIndex (not unique). GORM then issues DROP for the
// fixed uni_* name and fails with SQLSTATE 42704.
//
// Fix: drop any UNIQUE constraints on that column; uniqueness stays via the
// uniqueIndex GORM manages.
func FixAssistanceTokenUniqueConstraint(db *gorm.DB) error {
	var tableExists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'restaurant_tables'
		)
	`).Scan(&tableExists).Error; err != nil {
		return fmt.Errorf("check restaurant_tables: %w", err)
	}
	if !tableExists {
		return nil
	}

	var names []string
	if err := db.Raw(`
		SELECT tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_schema = kcu.constraint_schema
			AND tc.constraint_name = kcu.constraint_name
			AND tc.table_name = kcu.table_name
		WHERE tc.table_schema = 'public'
			AND tc.table_name = 'restaurant_tables'
			AND tc.constraint_type = 'UNIQUE'
			AND kcu.column_name = 'assistance_token'
		GROUP BY tc.constraint_name
		HAVING COUNT(*) = 1
	`).Scan(&names).Error; err != nil {
		return fmt.Errorf("list assistance_token unique constraints: %w", err)
	}

	for _, name := range names {
		sql := fmt.Sprintf(`ALTER TABLE restaurant_tables DROP CONSTRAINT IF EXISTS %q`, name)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("drop constraint %s: %w", name, err)
		}
		fmt.Printf("✅ Dropped unique constraint %s on restaurant_tables.assistance_token\n", name)
	}

	// Keep uniqueness via the index name GORM uniqueIndex typically uses.
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_tables_assistance_token
		ON restaurant_tables (assistance_token)
	`).Error; err != nil {
		return fmt.Errorf("ensure assistance_token unique index: %w", err)
	}

	return nil
}
