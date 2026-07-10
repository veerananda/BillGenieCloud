package migrations

import "gorm.io/gorm"

// AddPerformanceIndexes adds composite indexes for hot order and kitchen queries.
func AddPerformanceIndexes(db *gorm.DB) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_restaurant_created_at ON orders(restaurant_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_restaurant_status_created_at ON orders(restaurant_id, status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_order_items_order_status ON order_items(order_id, status)`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}
