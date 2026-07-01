package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const (
	// Legacy restaurants without subscription_config get generous grandfathered limits.
	legacyMaxManagers   = 1
	legacyMaxStaffChefs = 3
	legacyMaxTables     = 50
)

type SubscriptionLimits struct {
	IsLegacy              bool   `json:"is_legacy"`
	OperationMode         string `json:"operation_mode"`
	MaxTables             int    `json:"max_tables"`
	MaxManagers           int    `json:"max_managers"`
	MaxStaffAndChefs      int    `json:"max_staff_and_chefs"`
	HistoryDays           int    `json:"history_days"`
	KitchenDineIn         bool   `json:"kitchen_dine_in"`
	KitchenCounter bool `json:"kitchen_counter"`
	Inventory      bool `json:"inventory"`
	DineInEnabled  bool `json:"dine_in_enabled"`
	CounterEnabled        bool   `json:"counter_enabled"`
	MonthlyPrice          int    `json:"monthly_price"`
}

type SubscriptionUsage struct {
	Tables         int64 `json:"tables"`
	Managers       int64 `json:"managers"`
	StaffAndChefs  int64 `json:"staff_and_chefs"`
	Admins         int64 `json:"admins"`
}

type storedSubscriptionConfig struct {
	Selection SubscriptionSelection `json:"selection"`
	Quote     SubscriptionQuote     `json:"quote"`
}

func LoadSubscriptionLimits(db *gorm.DB, restaurant *models.Restaurant) (SubscriptionLimits, error) {
	if restaurant == nil {
		return SubscriptionLimits{}, errors.New("restaurant is required")
	}
	if len(restaurant.SubscriptionConfig) == 0 {
		return legacySubscriptionLimits(restaurant), nil
	}

	var stored storedSubscriptionConfig
	if err := json.Unmarshal(restaurant.SubscriptionConfig, &stored); err != nil {
		return legacySubscriptionLimits(restaurant), nil
	}

	sel, err := ValidateSubscriptionSelection(stored.Selection)
	if err != nil {
		return legacySubscriptionLimits(restaurant), nil
	}

	maxTables := 0
	if sel.OperationMode != "counter" {
		maxTables = NormalizeMaxTables(sel.MaxTables)
	}

	limits := SubscriptionLimits{
		OperationMode:         sel.OperationMode,
		MaxManagers:           BundledManagersFromTables(maxTables) + sel.ExtraManagers,
		MaxStaffAndChefs:      BundledStaffFromTables(maxTables) + sel.ExtraStaff,
		HistoryDays:           IncludedHistoryDaysINR,
		KitchenDineIn:  sel.KitchenDineIn,
		KitchenCounter: sel.KitchenCounter,
		Inventory:      sel.Inventory,
		MonthlyPrice:   restaurant.SubscriptionMonthlyPrice,
	}
	if sel.HistoryExtended {
		limits.HistoryDays = ExtendedHistoryDays
	}

	switch sel.OperationMode {
	case "counter":
		limits.CounterEnabled = true
		limits.DineInEnabled = false
		limits.MaxTables = 0
	case "both":
		limits.CounterEnabled = true
		limits.DineInEnabled = true
		limits.MaxTables = NormalizeMaxTables(sel.MaxTables)
	default:
		limits.DineInEnabled = true
		limits.CounterEnabled = false
		limits.MaxTables = NormalizeMaxTables(sel.MaxTables)
	}

	if limits.MonthlyPrice <= 0 {
		limits.MonthlyPrice = CalculateSubscriptionQuote(sel).MonthlySubtotal
	}

	return limits, nil
}

func legacySubscriptionLimits(restaurant *models.Restaurant) SubscriptionLimits {
	return SubscriptionLimits{
		IsLegacy:              true,
		OperationMode:         "both",
		MaxTables:             legacyMaxTables,
		MaxManagers:           legacyMaxManagers,
		MaxStaffAndChefs:      legacyMaxStaffChefs,
		HistoryDays:           ExtendedHistoryDays,
		KitchenDineIn:         true,
		KitchenCounter: true,
		Inventory:      true,
		DineInEnabled:  true,
		CounterEnabled:        true,
		MonthlyPrice:          restaurant.SubscriptionMonthlyPrice,
	}
}

func LoadSubscriptionUsage(db *gorm.DB, restaurantID string) (SubscriptionUsage, error) {
	var usage SubscriptionUsage
	if err := db.Model(&models.RestaurantTable{}).Where("restaurant_id = ?", restaurantID).Count(&usage.Tables).Error; err != nil {
		return usage, err
	}
	if err := db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "manager", true).Count(&usage.Managers).Error; err != nil {
		return usage, err
	}
	if err := db.Model(&models.User{}).Where("restaurant_id = ? AND role IN ? AND is_active = ?", restaurantID, []string{"staff", "chef"}, true).Count(&usage.StaffAndChefs).Error; err != nil {
		return usage, err
	}
	if err := db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "admin", true).Count(&usage.Admins).Error; err != nil {
		return usage, err
	}
	return usage, nil
}

func GetRestaurantSubscriptionBundle(db *gorm.DB, restaurantID string) (SubscriptionLimits, SubscriptionUsage, SubscriptionSelection, error) {
	var restaurant models.Restaurant
	if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return SubscriptionLimits{}, SubscriptionUsage{}, SubscriptionSelection{}, err
	}
	limits, err := LoadSubscriptionLimits(db, &restaurant)
	if err != nil {
		return SubscriptionLimits{}, SubscriptionUsage{}, SubscriptionSelection{}, err
	}
	usage, err := LoadSubscriptionUsage(db, restaurantID)
	if err != nil {
		return SubscriptionLimits{}, SubscriptionUsage{}, SubscriptionSelection{}, err
	}
	selection := DefaultSubscriptionSelection()
	if len(restaurant.SubscriptionConfig) > 0 {
		var stored storedSubscriptionConfig
		if err := json.Unmarshal(restaurant.SubscriptionConfig, &stored); err == nil {
			if validated, vErr := ValidateSubscriptionSelection(stored.Selection); vErr == nil {
				selection = validated
			}
		}
	}
	return limits, usage, selection, nil
}

func EnforceCreateTable(db *gorm.DB, restaurantID string) error {
	var restaurant models.Restaurant
	if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return err
	}
	limits, err := LoadSubscriptionLimits(db, &restaurant)
	if err != nil {
		return err
	}
	if !limits.DineInEnabled {
		return errors.New("dine-in is not included in your plan — upgrade to add tables")
	}
	if limits.MaxTables <= 0 {
		return errors.New("table capacity is not available on your plan")
	}
	var count int64
	if err := db.Model(&models.RestaurantTable{}).Where("restaurant_id = ?", restaurantID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) >= limits.MaxTables {
		return fmt.Errorf("table limit reached (%d/%d) — upgrade your plan to add more tables", count, limits.MaxTables)
	}
	return nil
}

func EnforceCreateUser(db *gorm.DB, restaurantID string, role string) error {
	var restaurant models.Restaurant
	if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return err
	}
	limits, err := LoadSubscriptionLimits(db, &restaurant)
	if err != nil {
		return err
	}

	var managers, staffChefs int64
	db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "manager", true).Count(&managers)
	db.Model(&models.User{}).Where("restaurant_id = ? AND role IN ? AND is_active = ?", restaurantID, []string{"staff", "chef"}, true).Count(&staffChefs)

	switch role {
	case "manager":
		if int(managers) >= limits.MaxManagers {
			return fmt.Errorf("manager limit reached (%d/%d) — increase table capacity or add manager seats in your subscription", managers, limits.MaxManagers)
		}
	case "chef":
		if !limits.KitchenDineIn && !limits.KitchenCounter {
			return errors.New("chef accounts require a kitchen add-on on your plan")
		}
		if int(staffChefs) >= limits.MaxStaffAndChefs {
			return fmt.Errorf("staff limit reached (%d/%d) — increase table capacity or add staff seats in your subscription", staffChefs, limits.MaxStaffAndChefs)
		}
	case "staff":
		if int(staffChefs) >= limits.MaxStaffAndChefs {
			return fmt.Errorf("staff limit reached (%d/%d) — increase table capacity or add staff seats in your subscription", staffChefs, limits.MaxStaffAndChefs)
		}
	default:
		return errors.New("invalid role")
	}
	return nil
}

func ValidateOrderCreate(limits SubscriptionLimits, req CreateOrderRequest) error {
	orderType := inferOrderType(req)
	switch orderType {
	case "counter":
		if !limits.CounterEnabled {
			return errors.New("counter / takeaway is not included in your plan")
		}
	case "dine_in":
		if !limits.DineInEnabled {
			return errors.New("dine-in orders are not included in your plan")
		}
	}
	return nil
}

func ClampHistoryFrom(limits SubscriptionLimits, requestedFrom time.Time) time.Time {
	earliest := time.Now().AddDate(0, 0, -limits.HistoryDays)
	if requestedFrom.Before(earliest) {
		return earliest
	}
	return requestedFrom
}

func OrderUsesKitchen(limits SubscriptionLimits, order *models.Order) bool {
	if order == nil {
		return false
	}
	if order.OrderType == "counter" || isLegacyCounterOrder(order) {
		return limits.KitchenCounter
	}
	return limits.KitchenDineIn
}

func isLegacyCounterOrder(order *models.Order) bool {
	if order.OrderType == "counter" {
		return true
	}
	switch order.CustomerName {
	case "Self Service", "Takeaway", "Counter":
		return true
	}
	if order.TableID != nil && len(*order.TableID) > 12 && (*order.TableID)[:12] == "self-service" {
		return true
	}
	return false
}

func EnforceKitchenUpdate(db *gorm.DB, restaurantID, orderID string) error {
	var restaurant models.Restaurant
	if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return err
	}
	limits, err := LoadSubscriptionLimits(db, &restaurant)
	if err != nil {
		return err
	}
	var order models.Order
	if err := db.Where("id = ? AND restaurant_id = ?", orderID, restaurantID).First(&order).Error; err != nil {
		return errors.New("order not found")
	}
	if !OrderUsesKitchen(limits, &order) {
		return errors.New("kitchen updates are not included in your plan for this order type")
	}
	return nil
}

func EnforceInventoryAccess(db *gorm.DB, restaurantID string) error {
	var restaurant models.Restaurant
	if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return err
	}
	limits, err := LoadSubscriptionLimits(db, &restaurant)
	if err != nil {
		return err
	}
	if !limits.Inventory {
		return errors.New("inventory management is not included in your plan")
	}
	return nil
}
