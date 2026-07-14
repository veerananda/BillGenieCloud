package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

func GenerateAssistanceToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func BuildAssistanceURL(token string) string {
	base := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if base == "" {
		base = "https://billgenie-api.fly.dev"
	}
	return base + "/a/" + token
}

func TableNeedsAssistance(table *models.RestaurantTable) bool {
	return table != nil && table.AssistanceRequestedAt != nil
}

func tableAssistanceToken(table *models.RestaurantTable) string {
	if table == nil || table.AssistanceToken == nil {
		return ""
	}
	return strings.TrimSpace(*table.AssistanceToken)
}

// EnsureTableAssistanceToken assigns a stable public token for the table QR if missing.
func EnsureTableAssistanceToken(db *gorm.DB, table *models.RestaurantTable) error {
	if table == nil {
		return errors.New("table not found")
	}
	if tableAssistanceToken(table) != "" {
		return nil
	}
	token, err := GenerateAssistanceToken()
	if err != nil {
		return err
	}
	if err := db.Model(table).Update("assistance_token", token).Error; err != nil {
		return err
	}
	table.AssistanceToken = &token
	return nil
}

func GetTableByAssistanceToken(db *gorm.DB, token string) (*models.RestaurantTable, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("invalid token")
	}
	var table models.RestaurantTable
	if err := db.Where("assistance_token = ?", token).First(&table).Error; err != nil {
		return nil, err
	}
	return &table, nil
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

// AssistanceStatus is the public polling payload for /a/:token/status.
type AssistanceStatus struct {
	RestaurantName      string  `json:"restaurant_name"`
	TableName           string  `json:"table_name"`
	IsOccupied          bool    `json:"is_occupied"`
	AssistanceRequested bool    `json:"assistance_requested"`
	HasActiveOrder      bool    `json:"has_active_order"`
	ItemCount           int     `json:"item_count"`
	BillAvailable       bool    `json:"bill_available"`
	BillURL             string  `json:"bill_url,omitempty"`
	BillDownloadURL     string  `json:"bill_download_url,omitempty"`
	OrderTotal          float64 `json:"order_total,omitempty"`
}
