package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// Keep in sync with BillGenieApp-new/src/config/subscriptionPricing.ts

const (
	BasicMonthlyPriceINR = 799

	PriceExtraStaffINR            = 99
	PriceExtraManagerINR          = 149
	PriceDualServiceINR           = 199
	PriceHistoryExtendedINR = 249
	PriceInventoryINR       = 349
	PriceKitchenDineInINR   = 299
	PriceKitchenCounterINR        = 199
	TableStaffBundlePriceINR      = 179
	TableStaffBundleSize          = 5
	TablesPerManager              = 15

	IncludedTablesBasic    = 10
	IncludedAdminsINR      = 1
	IncludedStaffINR       = 2
	IncludedHistoryDaysINR = 30
	ExtendedHistoryDays    = 730
	MaxTablesAllowed       = 50
	MinTablesDineIn        = 5
)

type SubscriptionSelection struct {
	BillingCycle          string `json:"billing_cycle"` // monthly | annual
	OperationMode         string `json:"operation_mode"`  // dine_in | counter | both
	MaxTables             int    `json:"max_tables"`      // dine-in table capacity (0 for counter-only)
	ExtraStaff            int    `json:"extra_staff"`
	ExtraManagers         int    `json:"extra_managers"`
	HistoryExtended bool `json:"history_extended"`
	Inventory       bool `json:"inventory"`
	KitchenDineIn   bool `json:"kitchen_dine_in"`
	KitchenCounter        bool   `json:"kitchen_counter"`
}

type SubscriptionLineItem struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Amount int    `json:"amount"`
}

type SubscriptionQuote struct {
	MonthlySubtotal         int                    `json:"monthly_subtotal"`
	AnnualTotal             int                    `json:"annual_total"`
	AnnualMonthlyEquivalent int                    `json:"annual_monthly_equivalent"`
	AnnualSavings           int                    `json:"annual_savings"`
	LineItems               []SubscriptionLineItem `json:"line_items"`
	Selection               SubscriptionSelection  `json:"selection"`
	BundledStaff            int                    `json:"bundled_staff"`
	BundledManagers         int                    `json:"bundled_managers"`
	TableBundles            int                    `json:"table_bundles"`
}

func DefaultSubscriptionSelection() SubscriptionSelection {
	return SubscriptionSelection{
		BillingCycle:  "monthly",
		OperationMode: "dine_in",
		MaxTables:     IncludedTablesBasic,
	}
}

func clampCount(value, max int) int {
	if value < 0 {
		return 0
	}
	if value > max {
		return max
	}
	return value
}

func TableBundlesAboveBasic(maxTables int) int {
	if maxTables <= IncludedTablesBasic {
		return 0
	}
	return (maxTables - IncludedTablesBasic) / TableStaffBundleSize
}

func BundledStaffFromTables(maxTables int) int {
	return IncludedStaffINR + TableBundlesAboveBasic(maxTables)
}

func BundledManagersFromTables(maxTables int) int {
	return maxTables / TablesPerManager
}

func NormalizeMaxTables(maxTables int) int {
	if maxTables <= IncludedTablesBasic {
		if maxTables < MinTablesDineIn {
			return MinTablesDineIn
		}
		if maxTables > IncludedTablesBasic {
			return IncludedTablesBasic
		}
		return maxTables
	}
	bundles := (maxTables - IncludedTablesBasic) / TableStaffBundleSize
	normalized := IncludedTablesBasic + bundles*TableStaffBundleSize
	if normalized > MaxTablesAllowed {
		return MaxTablesAllowed
	}
	if normalized < MinTablesDineIn {
		return MinTablesDineIn
	}
	return normalized
}

func ValidateSubscriptionSelection(sel SubscriptionSelection) (SubscriptionSelection, error) {
	switch sel.OperationMode {
	case "dine_in", "counter", "both":
	default:
		return sel, errors.New("operation_mode must be dine_in, counter, or both")
	}
	switch sel.BillingCycle {
	case "", "monthly":
		sel.BillingCycle = "monthly"
	case "annual":
	default:
		return sel, errors.New("billing_cycle must be monthly or annual")
	}
	sel.ExtraStaff = clampCount(sel.ExtraStaff, 50)
	sel.ExtraManagers = clampCount(sel.ExtraManagers, 20)
	if sel.OperationMode == "counter" {
		sel.MaxTables = 0
	} else {
		if sel.MaxTables <= 0 {
			sel.MaxTables = IncludedTablesBasic
		}
		sel.MaxTables = NormalizeMaxTables(sel.MaxTables)
	}
	return sel, nil
}

func CalculateSubscriptionQuote(sel SubscriptionSelection) SubscriptionQuote {
	sel, _ = ValidateSubscriptionSelection(sel)

	tableBundles := 0
	bundledStaff := IncludedStaffINR
	bundledManagers := 0
	if sel.OperationMode != "counter" {
		tableBundles = TableBundlesAboveBasic(sel.MaxTables)
		bundledStaff = BundledStaffFromTables(sel.MaxTables)
		bundledManagers = BundledManagersFromTables(sel.MaxTables)
	}

	lineItems := []SubscriptionLineItem{{
		ID:     "basic",
		Label:  fmt.Sprintf("Basic — 1 admin + %d staff, %d tables, menu, sales, 30-day history", IncludedStaffINR, IncludedTablesBasic),
		Amount: BasicMonthlyPriceINR,
	}}
	monthly := BasicMonthlyPriceINR

	if sel.OperationMode == "both" {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "dual_service", Label: "Dine-in + Counter (both service modes)", Amount: PriceDualServiceINR,
		})
		monthly += PriceDualServiceINR
	}
	if sel.OperationMode != "counter" {
		if tableBundles > 0 {
			amount := tableBundles * TableStaffBundlePriceINR
			lineItems = append(lineItems, SubscriptionLineItem{
				ID: "table_staff_bundles",
				Label: fmt.Sprintf("Table bundles × %d (+%d tables, +%d staff)", tableBundles, tableBundles*TableStaffBundleSize, tableBundles),
				Amount: amount,
			})
			monthly += amount
		}
		managerLabel := "managers"
		if bundledManagers == 1 {
			managerLabel = "manager"
		}
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "tables_capacity",
			Label: fmt.Sprintf("Table capacity — %d tables · %d staff · %d %s included", sel.MaxTables, bundledStaff, bundledManagers, managerLabel),
			Amount: 0,
		})
	}
	if sel.ExtraStaff > 0 {
		amount := sel.ExtraStaff * PriceExtraStaffINR
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "extra_staff", Label: fmt.Sprintf("Additional staff (beyond table bundle) × %d", sel.ExtraStaff), Amount: amount,
		})
		monthly += amount
	}
	if sel.ExtraManagers > 0 {
		amount := sel.ExtraManagers * PriceExtraManagerINR
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "extra_managers", Label: fmt.Sprintf("Additional managers (beyond table bundle) × %d", sel.ExtraManagers), Amount: amount,
		})
		monthly += amount
	}
	if sel.HistoryExtended {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "history_extended", Label: "Order history — 2 years", Amount: PriceHistoryExtendedINR,
		})
		monthly += PriceHistoryExtendedINR
	}
	if sel.Inventory {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "inventory", Label: "Inventory & stock updates", Amount: PriceInventoryINR,
		})
		monthly += PriceInventoryINR
	}
	if sel.KitchenDineIn {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "kitchen_dine_in", Label: "Kitchen updates — dine-in orders", Amount: PriceKitchenDineInINR,
		})
		monthly += PriceKitchenDineInINR
	}
	if sel.KitchenCounter {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "kitchen_counter", Label: "Kitchen updates — counter / takeaway", Amount: PriceKitchenCounterINR,
		})
		monthly += PriceKitchenCounterINR
	}

	annualTotal := monthly * 10
	return SubscriptionQuote{
		MonthlySubtotal:         monthly,
		AnnualTotal:             annualTotal,
		AnnualMonthlyEquivalent: int(math.Round(float64(annualTotal) / 12)),
		AnnualSavings:           monthly * 2,
		LineItems:               lineItems,
		Selection:               sel,
		BundledStaff:            bundledStaff + sel.ExtraStaff,
		BundledManagers:         bundledManagers + sel.ExtraManagers,
		TableBundles:            tableBundles,
	}
}

func SubscriptionConfigJSON(sel SubscriptionSelection, quote SubscriptionQuote) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"selection": sel,
		"quote":     quote,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes), nil
}

func IsBasicSubscriptionSelection(sel SubscriptionSelection) bool {
	if sel.OperationMode == "both" {
		return false
	}
	if sel.OperationMode != "dine_in" && sel.OperationMode != "counter" {
		return false
	}
	if sel.ExtraStaff != 0 || sel.ExtraManagers != 0 ||
		sel.HistoryExtended || sel.Inventory || sel.KitchenDineIn || sel.KitchenCounter {
		return false
	}
	if sel.OperationMode == "dine_in" {
		return sel.MaxTables == IncludedTablesBasic
	}
	return sel.MaxTables == 0
}

func SubscriptionPlanFromSelection(sel SubscriptionSelection) string {
	if IsBasicSubscriptionSelection(sel) {
		return "basic"
	}
	return "customised"
}

func ApplyOperationModeToRestaurant(isSelfService *bool, counterModes *string, mode string) {
	switch mode {
	case "counter":
		if isSelfService != nil {
			*isSelfService = true
		}
		if counterModes != nil {
			*counterModes = "both"
		}
	case "both":
		if isSelfService != nil {
			*isSelfService = false
		}
		if counterModes != nil {
			*counterModes = "both"
		}
	default: // dine_in
		if isSelfService != nil {
			*isSelfService = false
		}
		if counterModes != nil {
			*counterModes = "eat_here"
		}
	}
}
