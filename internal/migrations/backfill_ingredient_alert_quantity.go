package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

const alertQuantityBackfillMigrationID = "backfill_ingredient_alert_quantity_v1"

// BackfillIngredientAlertQuantity adds alert_quantity (if missing) and, exactly once,
// sets existing rows to 25% of full_stock when alert_quantity is still 0 and full_stock > 0.
//
// After the one-shot backfill is recorded, intentional alert_quantity = 0 (no alert)
// is never overwritten on later API restarts.
//
// For databases that already received the older every-boot backfill: if any row already
// has alert_quantity > 0, we only record the migration flag and skip further UPDATEs.
func BackfillIngredientAlertQuantity(db *gorm.DB) error {
	if err := db.Exec(`
		ALTER TABLE ingredients
		ADD COLUMN IF NOT EXISTS alert_quantity NUMERIC(10,2) NOT NULL DEFAULT 0
	`).Error; err != nil {
		return fmt.Errorf("add alert_quantity column: %w", err)
	}

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
		alertQuantityBackfillMigrationID,
	).Scan(&alreadyApplied).Error; err != nil {
		return fmt.Errorf("check alert quantity backfill flag: %w", err)
	}
	if alreadyApplied > 0 {
		fmt.Println("⏭️  BackfillIngredientAlertQuantity already applied; skipping")
		return nil
	}

	var existingAlerts int64
	if err := db.Raw(`SELECT COUNT(1) FROM ingredients WHERE alert_quantity > 0`).Scan(&existingAlerts).Error; err != nil {
		return fmt.Errorf("check existing alert quantities: %w", err)
	}

	var rowsAffected int64
	if existingAlerts > 0 {
		// Prior every-boot backfill (or manual edits) already populated alerts.
		// Do not rewrite intentional zeros.
		fmt.Println("⏭️  alert_quantity already populated; recording one-shot flag without re-backfill")
	} else {
		result := db.Exec(`
			UPDATE ingredients
			SET alert_quantity = ROUND((full_stock * 0.25)::numeric, 2)
			WHERE alert_quantity = 0 AND full_stock > 0
		`)
		if result.Error != nil {
			return fmt.Errorf("backfill alert_quantity: %w", result.Error)
		}
		rowsAffected = result.RowsAffected
	}

	if err := db.Exec(
		`INSERT INTO app_migrations (id) VALUES (?) ON CONFLICT (id) DO NOTHING`,
		alertQuantityBackfillMigrationID,
	).Error; err != nil {
		return fmt.Errorf("record alert quantity backfill: %w", err)
	}

	fmt.Printf("✅ BackfillIngredientAlertQuantity complete (updated %d row(s), one-shot recorded)\n", rowsAffected)
	return nil
}
