package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// Keep in sync with BillGenieApp-new/src/config/subscriptionPricing.ts
// and BillGenieWeb/src/data/pricing.ts

const (
	// Capacity bands (catalog). Custom deals may exceed MaxTablesAllowed.
	PlanStarterTables = 10
	PlanGrowthTables  = 18
	PlanScaleTables   = 25
	MaxTablesAllowed  = PlanScaleTables
	MaxTablesCustomDeal = 200
	MinTablesDineIn   = 5

	// Starter (≤10) monthly by city tier — marketing “starts from” = Tier 3.
	StarterMonthlyTier1INR = 1199
	StarterMonthlyTier2INR = 999
	StarterMonthlyTier3INR = 799

	// Growth (≤18)
	GrowthMonthlyTier1INR = 1499
	GrowthMonthlyTier2INR = 1299
	GrowthMonthlyTier3INR = 1099

	// Scale (≤25)
	ScaleMonthlyTier1INR = 1899
	ScaleMonthlyTier2INR = 1599
	ScaleMonthlyTier3INR = 1399

	// Compat aliases — Starter Tier-2 reference.
	BasicMonthlyPriceTier1INR = StarterMonthlyTier1INR
	BasicMonthlyPriceTier2INR = StarterMonthlyTier2INR
	BasicMonthlyPriceTier3INR = StarterMonthlyTier3INR
	BasicMonthlyPriceINR      = StarterMonthlyTier2INR
	IncludedTablesBasic       = PlanStarterTables

	PriceExtraStaffINR      = 69
	PriceExtraChefINR       = 69
	PriceExtraManagerINR    = 99
	PriceHistoryExtendedINR = 99
	PriceInventoryINR       = 299
	PriceExpensesINR        = 79

	MaxExtraStaff    = 5
	MaxExtraChefs    = 3
	MaxExtraManagers = 2

	IncludedAdminsINR      = 1
	IncludedManagersINR    = 1
	IncludedStaffINR       = 2
	IncludedChefsINR       = 1
	IncludedHistoryDaysINR = 90
	ExtendedHistoryDays    = 730
	AnnualMultiplier       = 11

	PlanBandStarter = "starter"
	PlanBandGrowth  = "growth"
	PlanBandScale   = "scale"

	PricingModeCatalog = "catalog"
	PricingModeCustom  = "custom"
)

type SubscriptionSelection struct {
	BillingCycle    string `json:"billing_cycle"`  // monthly | annual
	OperationMode   string `json:"operation_mode"` // always normalized to both for new plans
	MaxTables       int    `json:"max_tables"`     // snapped to 10 | 18 | 25 on catalog validate
	ExtraStaff      int    `json:"extra_staff"`
	ExtraChefs      int    `json:"extra_chefs"`
	ExtraManagers   int    `json:"extra_managers"`
	HistoryExtended bool   `json:"history_extended"`
	Inventory       bool   `json:"inventory"`
	Expenses        bool   `json:"expenses"`
	// Kitchen flags kept for backward-compatible JSON; always true on validate.
	KitchenDineIn  bool `json:"kitchen_dine_in"`
	KitchenCounter bool `json:"kitchen_counter"`
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
	BundledChefs            int                    `json:"bundled_chefs"`
	BundledManagers         int                    `json:"bundled_managers"`
	TableBundles            int                    `json:"table_bundles"` // always 0; kept for API compat
	CityTier                string                 `json:"city_tier"`
	PlanBand                string                 `json:"plan_band,omitempty"`
}

type TieredPricing struct {
	StarterMonthly  int `json:"starter_monthly"`
	GrowthMonthly   int `json:"growth_monthly"`
	ScaleMonthly    int `json:"scale_monthly"`
	BasicMonthly    int `json:"basic_monthly"` // alias = starter (compat)
	ExtraStaff      int `json:"extra_staff"`
	ExtraChef       int `json:"extra_chef"`
	ExtraManager    int `json:"extra_manager"`
	HistoryExtended int `json:"history_extended"`
	Inventory       int `json:"inventory"`
	Expenses        int `json:"expenses"`
}

func DefaultSubscriptionSelection() SubscriptionSelection {
	return SubscriptionSelection{
		BillingCycle:   "monthly",
		OperationMode:  "both",
		MaxTables:      PlanStarterTables,
		KitchenDineIn:  true,
		KitchenCounter: true,
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

// PlanBandFromTables maps table capacity to catalog band.
func PlanBandFromTables(maxTables int) string {
	if maxTables <= PlanStarterTables {
		return PlanBandStarter
	}
	if maxTables <= PlanGrowthTables {
		return PlanBandGrowth
	}
	return PlanBandScale
}

// TablesForPlanBand returns the included table ceiling for a band.
func TablesForPlanBand(band string) int {
	switch strings.ToLower(strings.TrimSpace(band)) {
	case PlanBandGrowth:
		return PlanGrowthTables
	case PlanBandScale:
		return PlanScaleTables
	default:
		return PlanStarterTables
	}
}

// NormalizeMaxTables snaps catalog capacity to Starter(10) / Growth(18) / Scale(25).
func NormalizeMaxTables(maxTables int) int {
	if maxTables <= 0 {
		return PlanStarterTables
	}
	if maxTables <= PlanStarterTables {
		return PlanStarterTables
	}
	if maxTables <= PlanGrowthTables {
		return PlanGrowthTables
	}
	return PlanScaleTables
}

func ValidateSubscriptionSelection(sel SubscriptionSelection) (SubscriptionSelection, error) {
	switch sel.BillingCycle {
	case "", "monthly":
		sel.BillingCycle = "monthly"
	case "annual":
	default:
		return sel, errors.New("billing_cycle must be monthly or annual")
	}

	sel.OperationMode = "both"
	sel.MaxTables = NormalizeMaxTables(sel.MaxTables)
	sel.KitchenDineIn = true
	sel.KitchenCounter = true

	sel.ExtraStaff = clampCount(sel.ExtraStaff, MaxExtraStaff)
	sel.ExtraChefs = clampCount(sel.ExtraChefs, MaxExtraChefs)
	sel.ExtraManagers = clampCount(sel.ExtraManagers, MaxExtraManagers)
	return sel, nil
}

func bandMonthlyForTier(band, tier string) int {
	tier = NormalizeTierLabel(tier)
	switch strings.ToLower(strings.TrimSpace(band)) {
	case PlanBandGrowth:
		switch tier {
		case CityTier1:
			return GrowthMonthlyTier1INR
		case CityTier3:
			return GrowthMonthlyTier3INR
		default:
			return GrowthMonthlyTier2INR
		}
	case PlanBandScale:
		switch tier {
		case CityTier1:
			return ScaleMonthlyTier1INR
		case CityTier3:
			return ScaleMonthlyTier3INR
		default:
			return ScaleMonthlyTier2INR
		}
	default: // starter
		switch tier {
		case CityTier1:
			return StarterMonthlyTier1INR
		case CityTier3:
			return StarterMonthlyTier3INR
		default:
			return StarterMonthlyTier2INR
		}
	}
}

func basicMonthlyForTier(tier string) int {
	return bandMonthlyForTier(PlanBandStarter, tier)
}

// PricingForTier returns band bases (tiered) + flat add-on prices.
func PricingForTier(tier string) TieredPricing {
	starter := bandMonthlyForTier(PlanBandStarter, tier)
	return TieredPricing{
		StarterMonthly:  starter,
		GrowthMonthly:   bandMonthlyForTier(PlanBandGrowth, tier),
		ScaleMonthly:    bandMonthlyForTier(PlanBandScale, tier),
		BasicMonthly:    starter,
		ExtraStaff:      PriceExtraStaffINR,
		ExtraChef:       PriceExtraChefINR,
		ExtraManager:    PriceExtraManagerINR,
		HistoryExtended: PriceHistoryExtendedINR,
		Inventory:       PriceInventoryINR,
		Expenses:        PriceExpensesINR,
	}
}

func CalculateSubscriptionQuoteForTier(sel SubscriptionSelection, tier string) SubscriptionQuote {
	sel, _ = ValidateSubscriptionSelection(sel)
	tier = NormalizeTierLabel(tier)
	band := PlanBandFromTables(sel.MaxTables)
	planPrice := bandMonthlyForTier(band, tier)
	pricing := PricingForTier(tier)

	bundledStaff := IncludedStaffINR + sel.ExtraStaff
	bundledChefs := IncludedChefsINR + sel.ExtraChefs
	bundledManagers := IncludedManagersINR + sel.ExtraManagers

	lineItems := []SubscriptionLineItem{{
		ID: "plan_" + band,
		Label: fmt.Sprintf(
			"%s — up to %d tables, dine-in + counter, kitchen, 1 admin + %d manager + %d staff + %d chef, %d-day history",
			planBandDisplayName(band), sel.MaxTables, IncludedManagersINR, IncludedStaffINR, IncludedChefsINR, IncludedHistoryDaysINR,
		),
		Amount: planPrice,
	}}
	monthly := planPrice

	if sel.ExtraStaff > 0 {
		amount := sel.ExtraStaff * pricing.ExtraStaff
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "extra_staff", Label: fmt.Sprintf("Additional staff × %d", sel.ExtraStaff), Amount: amount,
		})
		monthly += amount
	}
	if sel.ExtraChefs > 0 {
		amount := sel.ExtraChefs * pricing.ExtraChef
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "extra_chefs", Label: fmt.Sprintf("Additional chefs × %d", sel.ExtraChefs), Amount: amount,
		})
		monthly += amount
	}
	if sel.ExtraManagers > 0 {
		amount := sel.ExtraManagers * pricing.ExtraManager
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "extra_managers", Label: fmt.Sprintf("Additional managers × %d", sel.ExtraManagers), Amount: amount,
		})
		monthly += amount
	}
	if sel.HistoryExtended {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "history_extended", Label: "Order history — 2 years", Amount: pricing.HistoryExtended,
		})
		monthly += pricing.HistoryExtended
	}
	if sel.Inventory {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "inventory", Label: "Inventory suite (ingredients, stock, refill)", Amount: pricing.Inventory,
		})
		monthly += pricing.Inventory
	}
	if sel.Expenses {
		lineItems = append(lineItems, SubscriptionLineItem{
			ID: "expenses", Label: "Expenses page", Amount: pricing.Expenses,
		})
		monthly += pricing.Expenses
	}

	annualTotal := monthly * AnnualMultiplier
	return SubscriptionQuote{
		MonthlySubtotal:         monthly,
		AnnualTotal:             annualTotal,
		AnnualMonthlyEquivalent: int(math.Round(float64(annualTotal) / 12)),
		AnnualSavings:           monthly,
		LineItems:               lineItems,
		Selection:               sel,
		BundledStaff:            bundledStaff,
		BundledChefs:            bundledChefs,
		BundledManagers:         bundledManagers,
		TableBundles:            0,
		CityTier:                tier,
		PlanBand:                band,
	}
}

func CalculateSubscriptionQuote(sel SubscriptionSelection) SubscriptionQuote {
	return CalculateSubscriptionQuoteForTier(sel, CityTier2)
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

func IsBarePlanSelection(sel SubscriptionSelection) bool {
	sel, _ = ValidateSubscriptionSelection(sel)
	return sel.ExtraStaff == 0 &&
		sel.ExtraChefs == 0 &&
		sel.ExtraManagers == 0 &&
		!sel.HistoryExtended &&
		!sel.Inventory &&
		!sel.Expenses
}

// IsBasicSubscriptionSelection is kept for older call sites (= bare starter).
func IsBasicSubscriptionSelection(sel SubscriptionSelection) bool {
	sel, _ = ValidateSubscriptionSelection(sel)
	return PlanBandFromTables(sel.MaxTables) == PlanBandStarter && IsBarePlanSelection(sel)
}

func SubscriptionPlanFromSelection(sel SubscriptionSelection) string {
	sel, _ = ValidateSubscriptionSelection(sel)
	return PlanBandFromTables(sel.MaxTables)
}

func planBandDisplayName(band string) string {
	switch strings.ToLower(strings.TrimSpace(band)) {
	case PlanBandGrowth:
		return "Growth"
	case PlanBandScale:
		return "Scale"
	default:
		return "Starter"
	}
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
