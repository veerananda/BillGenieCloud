package services

import (
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const billShareTTL = 1 * time.Hour

// BillItemView is a single line on the public customer bill page.
type BillItemView struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	UnitRate float64 `json:"unit_rate"`
	Total    float64 `json:"total"`
}

// BillSummaryView is the public bill payload rendered for customers.
type BillSummaryView struct {
	RestaurantName   string         `json:"restaurant_name"`
	Address          string         `json:"address,omitempty"`
	ContactNumber    string         `json:"contact_number,omitempty"`
	TableNumber      string         `json:"table_number"`
	OrderNumber      int            `json:"order_number"`
	TicketNumber     int            `json:"ticket_number,omitempty"`
	ServiceMode      string         `json:"service_mode,omitempty"`
	CustomerName     string         `json:"customer_name,omitempty"`
	AttendedByName   string         `json:"attended_by_name,omitempty"`
	Items            []BillItemView `json:"items"`
	SubTotal         float64        `json:"sub_total"`
	TaxAmount        float64        `json:"tax_amount"`
	DiscountAmount   float64        `json:"discount_amount"`
	Total            float64        `json:"total"`
	IsPaid           bool           `json:"is_paid"`
	PaymentMethod    string         `json:"payment_method,omitempty"`
	PricesIncludeGST bool           `json:"prices_include_gst"`
	CompositeScheme  bool           `json:"composite_scheme"`
	CreatedAt        time.Time      `json:"created_at"`
}

func BuildBillURL(token string) string {
	base := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if base == "" {
		base = "https://billgenie-api.fly.dev"
	}
	return base + "/b/" + token
}

func orderItemsGross(items []models.OrderItem) float64 {
	taxable, nonTaxable := orderItemsGrossSplit(items)
	return taxable + nonTaxable
}

func resolveBillItemName(item models.OrderItem) string {
	name := "Item"
	if item.MenuItem != nil && item.MenuItem.Name != "" {
		name = item.MenuItem.Name
	}
	return FormatOrderItemDisplayName(name, item.VariantLabel)
}

// BuildBillSummary builds the customer-facing bill totals and line items.
func BuildBillSummary(order *models.Order, restaurant *models.Restaurant) BillSummaryView {
	discount := order.BillPreviewDiscount
	if order.Status == "completed" {
		discount = order.DiscountAmount
	}

	pricesIncludeGST := false
	compositeScheme := false
	restaurantName := ""
	address := ""
	contact := ""
	if restaurant != nil {
		pricesIncludeGST = restaurant.PricesIncludeGST
		compositeScheme = restaurant.CompositeScheme
		restaurantName = restaurant.Name
		address = restaurant.Address
		contact = restaurant.ContactNumber
		if contact == "" {
			contact = restaurant.Phone
		}
	}

	taxableGross, nonTaxableGross := orderItemsGrossSplit(order.Items)
	subTotal, taxAmount, total := CalculateRestaurantOrderTax(
		taxableGross,
		nonTaxableGross,
		discount,
		RestaurantTaxSettings{
			CompositeScheme:  compositeScheme,
			PricesIncludeGST: pricesIncludeGST,
		},
	)

	items := make([]BillItemView, 0, len(order.Items))
	for _, item := range order.Items {
		if item.Status == "cancelled" {
			continue
		}
		items = append(items, BillItemView{
			Name:     resolveBillItemName(item),
			Quantity: item.Quantity,
			UnitRate: item.UnitRate,
			Total:    item.Total,
		})
	}

	isPaid := order.Status == "completed"
	paymentMethod := ""
	if isPaid {
		paymentMethod = order.PaymentMethod
	}

	ticketNumber := order.TicketNumber
	if ticketNumber <= 0 {
		ticketNumber = order.OrderNumber
	}

	return BillSummaryView{
		RestaurantName:   restaurantName,
		Address:          address,
		ContactNumber:    contact,
		TableNumber:      order.TableNumber,
		OrderNumber:      order.OrderNumber,
		TicketNumber:     ticketNumber,
		ServiceMode:      order.ServiceMode,
		CustomerName:     order.CustomerName,
		AttendedByName:   AttendedByName(order),
		Items:            items,
		SubTotal:         subTotal,
		TaxAmount:        taxAmount,
		DiscountAmount:   discount,
		Total:            total,
		IsPaid:           isPaid,
		PaymentMethod:    paymentMethod,
		PricesIncludeGST: pricesIncludeGST,
		CompositeScheme:  compositeScheme,
		CreatedAt:        order.CreatedAt,
	}
}

// CreateBillShare assigns a bill token for customer QR sharing.
func (s *OrderService) CreateBillShare(restaurantID, orderID string, discountAmount float64) (*models.Order, error) {
	if discountAmount < 0 {
		discountAmount = 0
	}

	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	if order.Status == "cancelled" {
		return nil, errors.New("order is cancelled")
	}

	token, err := GenerateTrackingToken()
	if err != nil {
		return nil, err
	}

	expires := time.Now().Add(billShareTTL)
	updates := map[string]interface{}{
		"bill_token":            token,
		"bill_expires_at":       expires,
		"bill_preview_discount": discountAmount,
		"updated_at":            time.Now(),
	}

	if err := s.db.Model(&order).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.reloadOrderWithItems(orderID, restaurantID)
}

func (s *OrderService) GetOrderByBillToken(token string) (*models.Order, *models.Restaurant, error) {
	var order models.Order
	err := s.db.Preload("Items.MenuItem").
		Preload("AttendedBy").
		Where("bill_token = ?", token).
		First(&order).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, errors.New("bill link not found")
		}
		return nil, nil, err
	}

	if order.BillExpiresAt != nil && time.Now().After(*order.BillExpiresAt) {
		_ = s.clearBillShareToken(order.ID)
		return nil, nil, errors.New("bill link expired")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", order.RestaurantID).First(&restaurant).Error; err != nil {
		return &order, nil, nil
	}

	return &order, &restaurant, nil
}

func (s *OrderService) clearBillShareToken(orderID string) error {
	return s.db.Model(&models.Order{}).Where("id = ?", orderID).Updates(map[string]interface{}{
		"bill_token":            "",
		"bill_expires_at":       nil,
		"bill_preview_discount": 0,
		"updated_at":            time.Now(),
	}).Error
}

// ClearBillShare revokes a customer bill preview link for an order.
func (s *OrderService) ClearBillShare(restaurantID, orderID string) (*models.Order, error) {
	var order models.Order
	if err := s.db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}
	if err := s.clearBillShareToken(order.ID); err != nil {
		return nil, err
	}
	return s.reloadOrderWithItems(orderID, restaurantID)
}

// CleanupExpiredBillTokens removes bill share tokens that have passed their expiry.
func (s *OrderService) CleanupExpiredBillTokens() (int64, error) {
	result := s.db.Model(&models.Order{}).
		Where("bill_token <> '' AND bill_expires_at IS NOT NULL AND bill_expires_at < ?", time.Now()).
		Updates(map[string]interface{}{
			"bill_token":            "",
			"bill_expires_at":       nil,
			"bill_preview_discount": 0,
			"updated_at":            time.Now(),
		})
	return result.RowsAffected, result.Error
}

// StartBillTokenCleanup runs periodic cleanup of expired bill share links.
func (s *OrderService) StartBillTokenCleanup(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if count, err := s.CleanupExpiredBillTokens(); err != nil {
					log.Printf("bill token cleanup failed: %v", err)
				} else if count > 0 {
					log.Printf("cleared %d expired bill share token(s)", count)
				}
			}
		}
	}()
}
