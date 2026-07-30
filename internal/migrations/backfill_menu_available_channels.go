package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

const backfillMenuAvailableChannelsMigrationID = "backfill_menu_available_channels_v1"

const defaultAvailableChannelsJSON = `["dine_in","counter_eat_here","counter_takeaway","swiggy","zomato"]`

// BackfillMenuAvailableChannels sets available_channels to all sales channels
// for menu items where the column is null or an empty JSON array.
func BackfillMenuAvailableChannels(db *gorm.DB) error {
	var already int64
	if err := db.Table("schema_migrations").
		Where("id = ?", backfillMenuAvailableChannelsMigrationID).
		Count(&already).Error; err != nil {
		_ = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (id text PRIMARY KEY)`).Error
	} else if already > 0 {
		return nil
	}

	if err := db.Exec(`
		UPDATE menu_items
		SET available_channels = ?::jsonb
		WHERE available_channels IS NULL
		   OR available_channels::text IN ('null', '[]', '')
	`, defaultAvailableChannelsJSON).Error; err != nil {
		return fmt.Errorf("backfill menu available_channels: %w", err)
	}

	_ = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (id text PRIMARY KEY)`).Error
	if err := db.Exec(
		`INSERT INTO schema_migrations (id) VALUES (?) ON CONFLICT DO NOTHING`,
		backfillMenuAvailableChannelsMigrationID,
	).Error; err != nil {
		return fmt.Errorf("record available_channels backfill: %w", err)
	}
	return nil
}
