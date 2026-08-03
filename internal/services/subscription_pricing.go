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

	MaxExtraStaff    = 0 // catalog seats are fixed per band; extras only via custom deal
	MaxExtraChefs    = 0
	MaxExtraManagers = 0

	// Catalog seat bundles (totals, not add-ons).
	StarterStaffINR    = 2
	StarterChefsINR    = 1
	StarterManagersINR = 1
	GrowthStaffINR     = 4
	GrowthChefsINR     = 2
	GrowthManagersINR  = 1
	ScaleStaffINR      = 5
	ScaleChefsINR      = 3
	ScaleManagersINR   = 1

	IncludedAdminsINR      = 1
	IncludedManagersINR    = StarterManagersINR // starter baseline / trial
	IncludedStaffINR       = StarterStaffINR
	IncludedChefsINR       = StarterChefsINR
	IncludedHistoryDaysINR = 90
	ExtendedHistoryDays    = 730
	AnnualMultiplier       = 11 // pay 11 months, get 12
	// Catalog billing cycles (monthly removed).
	BillingCycleQuarterly  = "quarterly"
	BillingCycleHalfYearly = "half_yearly"
	BillingCycleAnnual     = "annual"
	QuarterlyMultiplier    = 3
	HalfYearlyMultiplier   = 6

	PlanBandStarter = "starter"
	PlanBandGrowth  = "growth"
	PlanBandScale   = "scale"

	PricingModeCatalog = "catalog"
	PricingModeCustom  = "custom"
)

type SubscriptionSelection struct {
	BillingCycle    string `json:"billing_cycle"`  // quarterly | half_yearly | annual
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
		BillingCycle:   BillingCycleQuarterly,
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

// BandSeatBundle returns included staff/chef/manager seats for a catalog table band.
func BandSeatBundle(maxTables int) (staff, chefs, managers int) {
	switch PlanBandFromTables(maxTables) {
	case PlanBandGrowth:
		return GrowthStaffINR, GrowthChefsINR, GrowthManagersINR
	case PlanBandScale:
		return ScaleStaffINR, ScaleChefsINR, ScaleManagersINR
	default:
		return StarterStaffINR, StarterChefsINR, StarterManagersINR
	}
}

func NormalizeBillingCycle(cycle string) string {
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case BillingCycleAnnual, "yearly", "year":
		return BillingCycleAnnual
	case BillingCycleHalfYearly, "half-yearly", "halfyearly", "semiannual", "semi_annual":
		return BillingCycleHalfYearly
	case BillingCycleQuarterly, "quarter", "":
		return BillingCycleQuarterly
	case "monthly", "month":
		// Legacy: monthly catalog billing removed — map to quarterly.
		return BillingCycleQuarterly
	default:
		return ""
	}
}

func BillingCycleMonths(cycle string) int {
	switch NormalizeBillingCycle(cycle) {
	case BillingCycleAnnual:
		return 12
	case BillingCycleHalfYearly:
		return 6
	default:
		return 3
	}
}

// PeriodSubtotalINR returns pre-GST amount for the billing period from a monthly quote base.
func PeriodSubtotalINR(quote SubscriptionQuote, billingCycle string) int {
	monthly := quote.MonthlySubtotal
	switch NormalizeBillingCycle(billingCycle) {
	case BillingCycleAnnual:
		if quote.AnnualTotal > 0 {
			return quote.AnnualTotal
		}
		return monthly * AnnualMultiplier
	case BillingCycleHalfYearly:
		return monthly * HalfYearlyMultiplier
	default:
		return monthly * QuarterlyMultiplier
	}
}

func BillingCycleLabel(cycle string) string {
	switch NormalizeBillingCycle(cycle) {
	case BillingCycleAnnual:
		return "year"
	case BillingCycleHalfYearly:
		return "6 months"
	default:
		return "quarter"
	}
}

func ValidateSubscriptionSelection(sel SubscriptionSelection) (SubscriptionSelection, error) {
	normalized := NormalizeBillingCycle(sel.BillingCycle)
	if normalized == "" && strings.TrimSpace(sel.BillingCycle) != "" {
		return sel, errors.New("billing_cycle must be quarterly, half_yearly, or annual")
	}
	if normalized == "" {
		normalized = BillingCycleQuarterly
	}
	sel.BillingCycle = normalized

	sel.OperationMode = "both"
	sel.MaxTables = NormalizeMaxTables(sel.MaxTables)
	sel.KitchenDineIn = true
	sel.KitchenCounter = true
	// Catalog plans bundle seats by band — self-serve extras are not sold.
	sel.ExtraStaff = 0
	sel.ExtraChefs = 0
	sel.ExtraManagers = 0
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

	bundledStaff, bundledChefs, bundledManagers := BandSeatBundle(sel.MaxTables)

	lineItems := []SubscriptionLineItem{{
		ID: "plan_" + band,
		Label: fmt.Sprintf(
			"%s — up to %d tables, dine-in + counter, kitchen, 1 admin + %d manager + %d staff + %d chef, %d-day history",
			planBandDisplayName(band), sel.MaxTables, bundledManagers, bundledStaff, bundledChefs, IncludedHistoryDaysINR,
		),
		Amount: planPrice,
	}}
	monthly := planPrice

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
