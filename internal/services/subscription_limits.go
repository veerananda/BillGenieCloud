package services

import (
	"errors"
	"fmt"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const (
	// Legacy restaurants without subscription_config get generous grandfathered limits.
	legacyMaxManagers   = 1
	legacyMaxStaff      = 3
	legacyMaxChefs      = 3
	legacyMaxStaffChefs = 6
	legacyMaxTables     = 50
)

type SubscriptionLimits struct {
	IsLegacy         bool   `json:"is_legacy"`
	OperationMode    string `json:"operation_mode"`
	MaxTables        int    `json:"max_tables"`
	MaxManagers      int    `json:"max_managers"`
	MaxStaff         int    `json:"max_staff"`
	MaxChefs         int    `json:"max_chefs"`
	MaxStaffAndChefs int    `json:"max_staff_and_chefs"` // MaxStaff + MaxChefs (compat)
	HistoryDays      int    `json:"history_days"`
	KitchenDineIn    bool   `json:"kitchen_dine_in"`
	KitchenCounter   bool   `json:"kitchen_counter"`
	Inventory        bool   `json:"inventory"`
	Expenses         bool   `json:"expenses"`
	DineInEnabled    bool   `json:"dine_in_enabled"`
	CounterEnabled   bool   `json:"counter_enabled"`
	MonthlyPrice     int    `json:"monthly_price"`
}

type SubscriptionUsage struct {
	Tables        int64 `json:"tables"`
	Managers      int64 `json:"managers"`
	Staff         int64 `json:"staff"`
	Chefs         int64 `json:"chefs"`
	StaffAndChefs int64 `json:"staff_and_chefs"`
	Admins        int64 `json:"admins"`
}

// LimitsFromSelection builds enforcement limits from a plan selection.
func LimitsFromSelection(sel SubscriptionSelection, monthlyPriceHint int) SubscriptionLimits {
	sel, _ = ValidateSubscriptionSelection(sel)

	maxStaff := IncludedStaffINR + sel.ExtraStaff
	maxChefs := IncludedChefsINR + sel.ExtraChefs
	limits := SubscriptionLimits{
		OperationMode:    "both",
		MaxTables:        sel.MaxTables,
		MaxManagers:      IncludedManagersINR + sel.ExtraManagers,
		MaxStaff:         maxStaff,
		MaxChefs:         maxChefs,
		MaxStaffAndChefs: maxStaff + maxChefs,
		HistoryDays:      IncludedHistoryDaysINR,
		KitchenDineIn:    true,
		KitchenCounter:   true,
		Inventory:        sel.Inventory,
		Expenses:         sel.Expenses,
		DineInEnabled:    true,
		CounterEnabled:   true,
		MonthlyPrice:     monthlyPriceHint,
	}
	if sel.HistoryExtended {
		limits.HistoryDays = ExtendedHistoryDays
	}
	if limits.MonthlyPrice <= 0 {
		limits.MonthlyPrice = CalculateSubscriptionQuote(sel).MonthlySubtotal
	}
	return limits
}

// UsageExceedsLimits reports whether current usage would violate the given limits.
func UsageExceedsLimits(usage SubscriptionUsage, limits SubscriptionLimits) error {
	if limits.DineInEnabled && limits.MaxTables > 0 && int(usage.Tables) > limits.MaxTables {
		return fmt.Errorf("you currently have %d tables but the new plan allows %d — remove tables before downgrading", usage.Tables, limits.MaxTables)
	}
	if !limits.DineInEnabled && usage.Tables > 0 {
		return fmt.Errorf("the new plan has no dine-in tables — remove existing tables before downgrading")
	}
	if int(usage.Managers) > limits.MaxManagers {
		return fmt.Errorf("you currently have %d managers but the new plan allows %d — remove managers before downgrading", usage.Managers, limits.MaxManagers)
	}
	if limits.MaxStaff > 0 && int(usage.Staff) > limits.MaxStaff {
		return fmt.Errorf("you currently have %d staff but the new plan allows %d — remove staff before downgrading", usage.Staff, limits.MaxStaff)
	}
	if limits.MaxChefs > 0 && int(usage.Chefs) > limits.MaxChefs {
		return fmt.Errorf("you currently have %d chefs but the new plan allows %d — remove chefs before downgrading", usage.Chefs, limits.MaxChefs)
	}
	if limits.MaxStaff == 0 && limits.MaxChefs == 0 && int(usage.StaffAndChefs) > limits.MaxStaffAndChefs {
		return fmt.Errorf("you currently have %d staff/chefs but the new plan allows %d — remove staff before downgrading", usage.StaffAndChefs, limits.MaxStaffAndChefs)
	}
	return nil
}

func LoadSubscriptionLimits(db *gorm.DB, restaurant *models.Restaurant) (SubscriptionLimits, error) {
	if restaurant == nil {
		return SubscriptionLimits{}, errors.New("restaurant is required")
	}
	if len(restaurant.SubscriptionConfig) == 0 {
		return legacySubscriptionLimits(restaurant), nil
	}

	stored := ParseStoredSubscriptionConfig(restaurant)
	if stored.Phase == SubscriptionPhaseTrial && time.Now().Before(restaurant.SubscriptionEnd) {
		limits := TrialSubscriptionLimits()
		limits.MonthlyPrice = restaurant.SubscriptionMonthlyPrice
		return limits, nil
	}

	return LimitsFromConfig(stored, restaurant.SubscriptionMonthlyPrice), nil
}

func legacySubscriptionLimits(restaurant *models.Restaurant) SubscriptionLimits {
	return SubscriptionLimits{
		IsLegacy:         true,
		OperationMode:    "both",
		MaxTables:        legacyMaxTables,
		MaxManagers:      legacyMaxManagers,
		MaxStaff:         legacyMaxStaff,
		MaxChefs:         legacyMaxChefs,
		MaxStaffAndChefs: legacyMaxStaffChefs,
		HistoryDays:      ExtendedHistoryDays,
		KitchenDineIn:    true,
		KitchenCounter:   true,
		Inventory:        true,
		Expenses:         true,
		DineInEnabled:    true,
		CounterEnabled:   true,
		MonthlyPrice:     restaurant.SubscriptionMonthlyPrice,
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
	if err := db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "staff", true).Count(&usage.Staff).Error; err != nil {
		return usage, err
	}
	if err := db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "chef", true).Count(&usage.Chefs).Error; err != nil {
		return usage, err
	}
	usage.StaffAndChefs = usage.Staff + usage.Chefs
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
		stored := ParseStoredSubscriptionConfig(&restaurant)
		selection = stored.Selection
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
		return fmt.Errorf("table limit reached (%d/%d). Your plan allows up to %d tables; need more? Ask for a custom plan", count, limits.MaxTables, limits.MaxTables)
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

	var managers, staffCount, chefCount int64
	db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "manager", true).Count(&managers)
	db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "staff", true).Count(&staffCount)
	db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "chef", true).Count(&chefCount)

	switch role {
	case "manager":
		if int(managers) >= limits.MaxManagers {
			return fmt.Errorf("manager limit reached (%d/%d) — add manager seats in your subscription", managers, limits.MaxManagers)
		}
	case "chef":
		if int(chefCount) >= limits.MaxChefs {
			return fmt.Errorf("chef limit reached (%d/%d) — add chef seats in your subscription", chefCount, limits.MaxChefs)
		}
	case "staff":
		if int(staffCount) >= limits.MaxStaff {
			return fmt.Errorf("staff limit reached (%d/%d) — add staff seats in your subscription", staffCount, limits.MaxStaff)
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

// IsLegacyCounterOrder reports whether an order is treated as counter/self-service.
func IsLegacyCounterOrder(order *models.Order) bool {
	return isLegacyCounterOrder(order)
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

func EnforceExpensesAccess(db *gorm.DB, restaurantID string) error {
	var restaurant models.Restaurant
	if err := db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return err
	}
	limits, err := LoadSubscriptionLimits(db, &restaurant)
	if err != nil {
		return err
	}
	if !limits.Expenses {
		return errors.New("expenses is not included in your plan — add the Expenses add-on")
	}
	return nil
}
