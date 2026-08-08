package services

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

func BuildAssistanceURL(token string) string {
	base := strings.TrimRight(os.Getenv("API_BASE_URL"), "/")
	if base == "" {
		base = "https://api.thebillgenie.com"
	}
	return base + "/a/" + token
}

// EnsureTableAssistanceToken mints a permanent public token for a dine-in table QR.
func EnsureTableAssistanceToken(db *gorm.DB, table *models.RestaurantTable) error {
	if table == nil {
		return errors.New("table not found")
	}
	if table.AssistanceToken != nil && strings.TrimSpace(*table.AssistanceToken) != "" {
		return nil
	}
	token, err := GenerateTrackingToken()
	if err != nil {
		return err
	}
	if err := db.Model(table).Update("assistance_token", token).Error; err != nil {
		return err
	}
	table.AssistanceToken = &token
	return nil
}

// RotateTableAssistanceToken replaces the table customer QR token.
// Use when reprinting a compromised sticker — not on every vacant (printed QRs stay fixed).
func RotateTableAssistanceToken(db *gorm.DB, tableID, restaurantID string) (string, error) {
	token, err := GenerateTrackingToken()
	if err != nil {
		return "", err
	}
	res := db.Model(&models.RestaurantTable{}).
		Where("id = ? AND restaurant_id = ?", tableID, restaurantID).
		Update("assistance_token", token)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", errors.New("table not found")
	}
	return token, nil
}

// EnsureOrderAssistanceToken kept for counter tracking / legacy callers.
// Dine-in customer QR now uses the fixed table token instead.
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

var (
	ErrTableVacant   = errors.New("table is vacant")
	ErrNoActiveOrder = errors.New("no active order")
)

// RequestTableAssistance sets the call-waiter flag. Returns true when this call newly raised it
// (so staff push can be sent once until cleared).
// Requires an occupied table with a live order (vacant / idle QR sessions cannot alert staff).
func RequestTableAssistance(db *gorm.DB, table *models.RestaurantTable) (newlyRequested bool, err error) {
	if table == nil {
		return false, errors.New("table not found")
	}
	if !table.IsOccupied {
		return false, ErrTableVacant
	}
	if table.CurrentOrderID == nil || strings.TrimSpace(*table.CurrentOrderID) == "" {
		return false, ErrNoActiveOrder
	}
	if table.AssistanceRequestedAt != nil {
		return false, nil
	}
	now := time.Now()
	if err := db.Model(table).Update("assistance_requested_at", now).Error; err != nil {
		return false, err
	}
	table.AssistanceRequestedAt = &now
	return true, nil
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
}

// AssistanceMenuVariant is a lean dine-in menu portion for the customer page.
type AssistanceMenuVariant struct {
	Label string  `json:"label"`
	Price float64 `json:"price"`
}

// AssistanceMenuItem is a lean dine-in menu row (browse only).
type AssistanceMenuItem struct {
	Name        string                  `json:"name"`
	Category    string                  `json:"category,omitempty"`
	Description string                  `json:"description,omitempty"`
	IsVeg       bool                    `json:"is_veg"`
	Price       float64                 `json:"price"`
	Variants    []AssistanceMenuVariant `json:"variants,omitempty"`
}

// AssistanceStatus is the public SSE/status payload for /a/:token.
// Phase: idle | seated | checkout — driven by live table state, not client cache.
type AssistanceStatus struct {
	RestaurantName      string `json:"restaurant_name"`
	TableName           string `json:"table_name"`
	Phase               string `json:"phase"` // idle | seated | checkout
	IsOccupied          bool   `json:"is_occupied"`
	AssistanceRequested bool   `json:"assistance_requested"`
	CallWaiterAllowed   bool   `json:"call_waiter_allowed"`
	HasActiveOrder      bool   `json:"has_active_order"`
	MenuVisible         bool   `json:"menu_visible"`
	OrderStatus         string `json:"order_status,omitempty"`
	ItemCount           int    `json:"item_count"`
	Items               []AssistanceBillItem `json:"items,omitempty"`
	BillAvailable       bool                 `json:"bill_available"`
	BillURL             string               `json:"bill_url,omitempty"`
	BillDownloadURL     string               `json:"bill_download_url,omitempty"`
	BillExpiresAt       *time.Time           `json:"bill_expires_at,omitempty"`
	OrderTotal          float64              `json:"order_total,omitempty"`
	// Bill review totals (populated when bill_available).
	SubTotal         float64 `json:"sub_total"`
	TaxAmount        float64 `json:"tax_amount"`
	DiscountAmount   float64 `json:"discount_amount"`
	PricesIncludeGST bool    `json:"prices_include_gst"`
	CompositeScheme  bool    `json:"composite_scheme"`
	ShowTax          bool    `json:"show_tax"`
}

// ResolveTableByAssistanceToken finds a table by its permanent QR token.
// Falls back to a legacy order tracking token and returns that order's table
// so older printed QRs still open the live table session.
func ResolveTableByAssistanceToken(db *gorm.DB, token string) (*models.RestaurantTable, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var table models.RestaurantTable
	err := db.Where("assistance_token = ?", token).First(&table).Error
	if err == nil {
		return &table, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var order models.Order
	if err := db.Where("tracking_token = ?", token).First(&order).Error; err != nil {
		return nil, err
	}
	if order.TableID == nil || strings.TrimSpace(*order.TableID) == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if err := db.Where("id = ? AND restaurant_id = ?", *order.TableID, order.RestaurantID).First(&table).Error; err != nil {
		return nil, err
	}
	return &table, nil
}

// BuildAssistanceStatusForTable builds the live customer page payload for a table.
// Bill review is only exposed while the table is still linked to that checkout order;
// after clear/payment the table page returns to idle (bill download link may still
// work via /b/{billToken} until billShareTTL).
func BuildAssistanceStatusForTable(db *gorm.DB, orderService *OrderService, table *models.RestaurantTable) (*AssistanceStatus, error) {
	if table == nil {
		return nil, errors.New("table not found")
	}

	status := &AssistanceStatus{
		TableName:  table.Name,
		IsOccupied: table.IsOccupied,
		// Call waiter only for the live dine-in order (not vacant, not "occupied" with no order).
		CallWaiterAllowed: false,
		Phase:             "idle",
		MenuVisible:       true,
	}
	status.AssistanceRequested = TableNeedsAssistance(table)

	var restaurant models.Restaurant
	if err := db.Where("id = ?", table.RestaurantID).First(&restaurant).Error; err == nil {
		status.RestaurantName = restaurant.Name
	}

	var order *models.Order
	if table.CurrentOrderID != nil && strings.TrimSpace(*table.CurrentOrderID) != "" && orderService != nil {
		if o, err := orderService.GetOrderByID(table.RestaurantID, *table.CurrentOrderID); err == nil && o != nil {
			if o.Status != "completed" && o.Status != "cancelled" {
				order = o
			}
		}
	}

	if order == nil {
		return status, nil
	}

	status.HasActiveOrder = true
	status.IsOccupied = true
	status.CallWaiterAllowed = true
	status.OrderStatus = order.Status
	status.Phase = "seated"
	status.OrderTotal = order.Total
	if order.Total <= 0 {
		status.OrderTotal = order.SubTotal
	}

	billLive := strings.TrimSpace(order.BillToken) != "" &&
		(order.BillExpiresAt == nil || order.BillExpiresAt.After(time.Now()))
	if !billLive {
		return status, nil
	}

	// Checkout / bill review phase — hide menu, show bill.
	status.Phase = "checkout"
	status.MenuVisible = false
	status.BillAvailable = true
	status.BillURL = BuildBillURL(order.BillToken)
	status.BillDownloadURL = status.BillURL + "/download"
	status.BillExpiresAt = order.BillExpiresAt

	var restPtr *models.Restaurant
	if restaurant.ID != "" {
		restPtr = &restaurant
	}
	summary := BuildBillSummary(order, restPtr)
	status.SubTotal = summary.SubTotal
	status.TaxAmount = summary.TaxAmount
	status.DiscountAmount = summary.DiscountAmount
	status.OrderTotal = summary.Total
	status.PricesIncludeGST = summary.PricesIncludeGST
	status.CompositeScheme = summary.CompositeScheme
	status.ShowTax = summary.TaxAmount > 0 && !summary.CompositeScheme

	groupedItems := make(map[string]int)
	blocklist := []string(nil)
	if restPtr != nil {
		blocklist = ParseCategoryDisplayBlocklist(restPtr.CategoryDisplayBlocklist)
	}
	for _, item := range order.Items {
		if item.Status == "cancelled" {
			continue
		}
		status.ItemCount += item.Quantity
		unitRate := item.UnitRate
		if unitRate <= 0 && item.Quantity > 0 {
			unitRate = item.Total / float64(item.Quantity)
		}
		name := strings.TrimSpace(item.MenuID)
		category := ""
		if item.MenuItem != nil {
			if strings.TrimSpace(item.MenuItem.Name) != "" {
				name = item.MenuItem.Name
			}
			category = item.MenuItem.Category
		}
		name = FormatItemDisplayName(name, category, item.VariantLabel, blocklist)
		displayCategory := ""
		if IsBlockedDisplayCategory(category, blocklist) {
			displayCategory = category
		}
		lineTotal := item.Total
		if lineTotal <= 0 {
			lineTotal = unitRate * float64(item.Quantity)
		}
		variantKey := ""
		if item.VariantID != nil {
			variantKey = *item.VariantID
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%.2f", item.MenuID, variantKey, strings.ToLower(name), strings.ToLower(displayCategory), unitRate)
		if idx, ok := groupedItems[key]; ok {
			status.Items[idx].Quantity += item.Quantity
			status.Items[idx].Total += lineTotal
			continue
		}
		groupedItems[key] = len(status.Items)
		status.Items = append(status.Items, AssistanceBillItem{
			Name:     name,
			Quantity: item.Quantity,
			UnitRate: unitRate,
			Total:    lineTotal,
			Category: displayCategory,
		})
	}

	return status, nil
}

func menuItemOnDineInChannel(item *models.MenuItem) bool {
	if item == nil || !item.IsAvailable {
		return false
	}
	if len(item.AvailableChannels) == 0 {
		return true
	}
	for _, ch := range item.AvailableChannels {
		if ch == "dine_in" {
			return true
		}
	}
	return false
}

func dineInUnitPrice(item *models.MenuItem) float64 {
	if item == nil {
		return 0
	}
	if item.ChannelPrices != nil {
		if p, ok := item.ChannelPrices["dine_in"]; ok && p >= 0 {
			return p
		}
	}
	return item.Price
}

func dineInVariantPrice(v *models.MenuItemVariant, item *models.MenuItem) float64 {
	if v == nil {
		return dineInUnitPrice(item)
	}
	if v.ChannelPrices != nil {
		if p, ok := v.ChannelPrices["dine_in"]; ok && p >= 0 {
			return p
		}
	}
	if v.IsDefault {
		return dineInUnitPrice(item)
	}
	return v.Price
}

// LoadAssistanceMenu returns browse-only dine-in menu items for the table's restaurant.
// Item names use FormatItemDisplayName (same smart naming as bills/KOTs).
func LoadAssistanceMenu(db *gorm.DB, restaurantID string) ([]AssistanceMenuItem, error) {
	var restaurant models.Restaurant
	_ = db.Select("id", "category_display_blocklist").Where("id = ?", restaurantID).First(&restaurant).Error
	blocklist := ParseCategoryDisplayBlocklist(restaurant.CategoryDisplayBlocklist)

	var items []models.MenuItem
	if err := db.Preload("Variants", func(tx *gorm.DB) *gorm.DB {
		return tx.Where("is_available = ?", true).Order("sort_order ASC, created_at ASC")
	}).Where("restaurant_id = ? AND is_available = ?", restaurantID, true).
		Order("category ASC, name ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	out := make([]AssistanceMenuItem, 0, len(items))
	for i := range items {
		item := &items[i]
		if !menuItemOnDineInChannel(item) {
			continue
		}
		row := AssistanceMenuItem{
			Name:        FormatItemDisplayName(item.Name, item.Category, "", blocklist),
			Category:    item.Category,
			Description: strings.TrimSpace(item.Description),
			IsVeg:       item.IsVeg,
			Price:       dineInUnitPrice(item),
		}
		for j := range item.Variants {
			v := &item.Variants[j]
			row.Variants = append(row.Variants, AssistanceMenuVariant{
				Label: v.Label,
				Price: dineInVariantPrice(v, item),
			})
		}
		out = append(out, row)
	}
	return out, nil
}
