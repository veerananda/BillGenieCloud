package services

import (
	"errors"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

func BuildAssistanceURL(token string) string {
	base := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if base == "" {
		base = "https://billgenie-api.fly.dev"
	}
	return base + "/a/" + token
}

// EnsureOrderAssistanceToken assigns a per-order public token for the dine-in assistance page.
func EnsureOrderAssistanceToken(db *gorm.DB, order *models.Order) error {
	if order == nil {
		return errors.New("order not found")
	}
	if strings.TrimSpace(order.TrackingToken) != "" {
		return nil
	}
	token, err := GenerateTrackingToken()
	if err != nil {
		return err
	}
	expires := time.Now().Add(trackingTTL)
	if err := db.Model(order).Updates(map[string]interface{}{
		"tracking_token":      token,
		"tracking_expires_at": expires,
		"updated_at":          time.Now(),
	}).Error; err != nil {
		return err
	}
	order.TrackingToken = token
	order.TrackingExpiresAt = &expires
	return nil
}

func TableNeedsAssistance(table *models.RestaurantTable) bool {
	return table != nil && table.AssistanceRequestedAt != nil
}

func RequestTableAssistance(db *gorm.DB, table *models.RestaurantTable) error {
	now := time.Now()
	if err := db.Model(table).Update("assistance_requested_at", now).Error; err != nil {
		return err
	}
	table.AssistanceRequestedAt = &now
	return nil
}

func ClearTableAssistance(db *gorm.DB, table *models.RestaurantTable) error {
	if table == nil {
		return nil
	}
	if err := db.Model(table).Update("assistance_requested_at", nil).Error; err != nil {
		return err
	}
	table.AssistanceRequestedAt = nil
	return nil
}

// AssistanceBillItem is the public bill line item shown on the assistance page.
type AssistanceBillItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	UnitRate float64 `json:"unit_rate"`
	Total    float64 `json:"total"`
	Category string  `json:"category,omitempty"`
	IsVeg    *bool   `json:"is_veg,omitempty"`
}

// AssistanceStatus is the public SSE/status payload for /a/:token.
type AssistanceStatus struct {
	RestaurantName      string               `json:"restaurant_name"`
	TableName           string               `json:"table_name"`
	IsOccupied          bool                 `json:"is_occupied"`
	AssistanceRequested bool                 `json:"assistance_requested"`
	HasActiveOrder      bool                 `json:"has_active_order"`
	OrderStatus         string               `json:"order_status,omitempty"`
	ItemCount           int                  `json:"item_count"`
	Items               []AssistanceBillItem `json:"items,omitempty"`
	BillAvailable       bool                 `json:"bill_available"`
	BillURL             string               `json:"bill_url,omitempty"`
	BillDownloadURL     string               `json:"bill_download_url,omitempty"`
	OrderTotal          float64              `json:"order_total,omitempty"`
}
