package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const trackingTTL = 4 * time.Hour

// TrackingStatus is the public-facing order readiness state for customer QR pages.
type TrackingStatus struct {
	TicketNumber   int    `json:"ticket_number"`
	OrderNumber    int    `json:"order_number"`
	RestaurantName string `json:"restaurant_name,omitempty"`
	ServiceMode    string `json:"service_mode,omitempty"`
	Color          string `json:"color"`   // red | orange | green
	Message        string `json:"message"` // Preparing | N of M items ready | Ready for pickup
	ReadyCount     int    `json:"ready_count"`
	TotalCount     int    `json:"total_count"`
	Status         string `json:"status"` // order status
}

func GenerateTrackingToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func BuildTrackingURL(token string) string {
	base := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if base == "" {
		base = "https://billgenie-api.fly.dev"
	}
	return base + "/t/" + token
}

func IsCounterOrderForTracking(order *models.Order) bool {
	if order == nil {
		return false
	}
	if order.OrderType == "counter" {
		return true
	}
	switch order.CustomerName {
	case "Self Service", "Takeaway", "Counter":
		return true
	}
	if order.TableID != nil && strings.HasPrefix(*order.TableID, "self-service") {
		return true
	}
	return false
}

func BuildTrackingStatus(order *models.Order, restaurantName string) TrackingStatus {
	readyCount := 0
	totalCount := 0
	for _, item := range order.Items {
		if item.Status == "cancelled" {
			continue
		}
		totalCount++
		if item.Status == "ready" || item.Status == "served" {
			readyCount++
		}
	}

	color := "red"
	message := "Preparing your order"
	if totalCount == 0 {
		color = "green"
		message = "Order updated"
	} else if readyCount > 0 && readyCount < totalCount {
		color = "orange"
		message = fmt.Sprintf("%d of %d items ready", readyCount, totalCount)
	} else if readyCount >= totalCount {
		color = "green"
		message = "Ready for pickup"
	}

	ticket := order.TicketNumber
	if ticket <= 0 {
		ticket = order.OrderNumber
	}

	return TrackingStatus{
		TicketNumber:   ticket,
		OrderNumber:    order.OrderNumber,
		RestaurantName: restaurantName,
		ServiceMode:    order.ServiceMode,
		Color:          color,
		Message:        message,
		ReadyCount:     readyCount,
		TotalCount:     totalCount,
		Status:         order.Status,
	}
}

func (s *OrderService) AssignCounterTrackingToken(orderID, restaurantID string) (*models.Order, error) {
	token, err := GenerateTrackingToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(trackingTTL)
	if err := s.db.Model(&models.Order{}).
		Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		Updates(map[string]interface{}{
			"tracking_token":      token,
			"tracking_expires_at": expires,
			"updated_at":          time.Now(),
		}).Error; err != nil {
		return nil, err
	}
	return s.GetOrderByID(restaurantID, orderID)
}

func (s *OrderService) GetOrderByTrackingToken(token string) (*models.Order, *models.Restaurant, error) {
	var order models.Order
	err := s.db.Preload("Items.MenuItem").
		Where("tracking_token = ?", token).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("tracking link not found")
		}
		return nil, nil, err
	}

	if order.TrackingExpiresAt != nil && time.Now().After(*order.TrackingExpiresAt) {
		return nil, nil, errors.New("tracking link expired")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", order.RestaurantID).First(&restaurant).Error; err != nil {
		return &order, nil, nil
	}

	return &order, &restaurant, nil
}

func (s *OrderService) GetRestaurantName(restaurantID string) string {
	var restaurant models.Restaurant
	if err := s.db.Select("name").Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return ""
	}
	return restaurant.Name
}
