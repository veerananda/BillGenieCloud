package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// CustomLimitsOverride raises catalog caps for enterprise deals (platform-only).
type CustomLimitsOverride struct {
	MaxTables    *int  `json:"max_tables,omitempty"`
	MaxStaff     *int  `json:"max_staff,omitempty"`
	MaxChefs     *int  `json:"max_chefs,omitempty"`
	MaxManagers  *int  `json:"max_managers,omitempty"`
	HistoryDays  *int  `json:"history_days,omitempty"`
	Inventory    *bool `json:"inventory,omitempty"`
	Expenses     *bool `json:"expenses,omitempty"`
	KitchenDine  *bool `json:"kitchen_dine_in,omitempty"`
	KitchenCtr   *bool `json:"kitchen_counter,omitempty"`
	DineIn       *bool `json:"dine_in_enabled,omitempty"`
	Counter      *bool `json:"counter_enabled,omitempty"`
}

// CustomDeal is a per-restaurant commercial override set from platform ops.
type CustomDeal struct {
	MonthlyPrice         int                    `json:"monthly_price"`
	AnnualPrice          int                    `json:"annual_price,omitempty"` // 0 = monthly * AnnualMultiplier
	Selection            SubscriptionSelection  `json:"selection"`
	LimitsOverride       *CustomLimitsOverride  `json:"limits_override,omitempty"`
	LockSelfServeChanges bool                   `json:"lock_self_serve_changes"`
	Notes                string                 `json:"notes,omitempty"`
	SetBy                string                 `json:"set_by,omitempty"`
	SetAt                *time.Time             `json:"set_at,omitempty"`
}

const (
	CustomDealRequestPending   = "pending"
	CustomDealRequestFulfilled = "fulfilled"
	CustomDealRequestCancelled = "cancelled"
)

// CustomDealRequest is submitted from app/web when the restaurant needs a negotiated plan.
// Price is never set by the customer — platform ops applies a CustomDeal after review.
type CustomDealRequest struct {
	MaxTables       int        `json:"max_tables"`
	ExtraStaff      int        `json:"extra_staff"`
	ExtraChefs      int        `json:"extra_chefs"`
	ExtraManagers   int        `json:"extra_managers"`
	Inventory       bool       `json:"inventory"`
	Expenses        bool       `json:"expenses"`
	HistoryExtended bool       `json:"history_extended"`
	BillingCycle    string     `json:"billing_cycle"`
	Notes           string     `json:"notes,omitempty"`
	ContactPhone    string     `json:"contact_phone,omitempty"`
	Status          string     `json:"status"`
	RequestedAt     *time.Time `json:"requested_at,omitempty"`
}

func HasPendingCustomDealRequest(cfg StoredSubscriptionConfig) bool {
	return cfg.CustomDealRequest != nil &&
		strings.EqualFold(strings.TrimSpace(cfg.CustomDealRequest.Status), CustomDealRequestPending)
}

func ValidateCustomDealRequest(req CustomDealRequest) (CustomDealRequest, error) {
	switch req.BillingCycle {
	case "", "monthly":
		req.BillingCycle = "monthly"
	case "annual":
	default:
		return req, errors.New("billing_cycle must be monthly or annual")
	}
	if req.MaxTables < PlanScaleTables+1 {
		req.MaxTables = PlanScaleTables + 1 // custom starts above catalog Scale (25)
	}
	if req.MaxTables > MaxTablesCustomDeal {
		req.MaxTables = MaxTablesCustomDeal
	}
	req.ExtraStaff = clampCount(req.ExtraStaff, 100)
	req.ExtraChefs = clampCount(req.ExtraChefs, 50)
	req.ExtraManagers = clampCount(req.ExtraManagers, 50)
	req.Notes = strings.TrimSpace(req.Notes)
	req.ContactPhone = strings.TrimSpace(req.ContactPhone)
	if len(req.Notes) > 2000 {
		return req, errors.New("notes must be at most 2000 characters")
	}
	if req.Status == "" {
		req.Status = CustomDealRequestPending
	}
	return req, nil
}

func SelectionFromCustomDealRequest(req CustomDealRequest) SubscriptionSelection {
	sel := SubscriptionSelection{
		BillingCycle:    req.BillingCycle,
		OperationMode:   "both",
		MaxTables:       req.MaxTables,
		ExtraStaff:      req.ExtraStaff,
		ExtraChefs:      req.ExtraChefs,
		ExtraManagers:   req.ExtraManagers,
		HistoryExtended: req.HistoryExtended,
		Inventory:       req.Inventory,
		Expenses:        req.Expenses,
		KitchenDineIn:   true,
		KitchenCounter:  true,
	}
	validated, err := ValidateCustomDealSelection(sel)
	if err != nil {
		return sel
	}
	return validated
}

func (cfg StoredSubscriptionConfig) IsCustomDeal() bool {
	mode := strings.ToLower(strings.TrimSpace(cfg.PricingMode))
	return mode == PricingModeCustom && cfg.CustomDeal != nil && cfg.CustomDeal.MonthlyPrice > 0
}

func (cfg StoredSubscriptionConfig) EffectiveSelection() SubscriptionSelection {
	if cfg.IsCustomDeal() {
		return cfg.CustomDeal.Selection
	}
	return cfg.Selection
}

// ValidateCustomDealSelection allows higher table/seat caps than catalog Basic.
func ValidateCustomDealSelection(sel SubscriptionSelection) (SubscriptionSelection, error) {
	switch sel.BillingCycle {
	case "", "monthly":
		sel.BillingCycle = "monthly"
	case "annual":
	default:
		return sel, errors.New("billing_cycle must be monthly or annual")
	}
	sel.OperationMode = "both"
	sel.KitchenDineIn = true
	sel.KitchenCounter = true
	if sel.MaxTables <= 0 {
		sel.MaxTables = PlanStarterTables
	}
	if sel.MaxTables > MaxTablesCustomDeal {
		sel.MaxTables = MaxTablesCustomDeal
	}
	if sel.MaxTables < MinTablesDineIn {
		sel.MaxTables = MinTablesDineIn
	}
	sel.ExtraStaff = clampCount(sel.ExtraStaff, 100)
	sel.ExtraChefs = clampCount(sel.ExtraChefs, 50)
	sel.ExtraManagers = clampCount(sel.ExtraManagers, 50)
	return sel, nil
}

func ValidateCustomDeal(deal CustomDeal) (CustomDeal, error) {
	if deal.MonthlyPrice < 1 {
		return deal, errors.New("monthly_price must be at least 1")
	}
	if deal.MonthlyPrice > 500000 {
		return deal, errors.New("monthly_price is unreasonably high")
	}
	sel, err := ValidateCustomDealSelection(deal.Selection)
	if err != nil {
		return deal, err
	}
	deal.Selection = sel
	if deal.AnnualPrice < 0 {
		deal.AnnualPrice = 0
	}
	if deal.LimitsOverride != nil {
		o := deal.LimitsOverride
		if o.MaxTables != nil {
			v := *o.MaxTables
			if v < MinTablesDineIn {
				v = MinTablesDineIn
			}
			if v > MaxTablesCustomDeal {
				v = MaxTablesCustomDeal
			}
			o.MaxTables = &v
		}
		for _, ptr := range []*int{o.MaxStaff, o.MaxChefs, o.MaxManagers} {
			if ptr != nil && *ptr < 0 {
				return deal, errors.New("seat overrides cannot be negative")
			}
		}
		if o.HistoryDays != nil {
			h := *o.HistoryDays
			if h < IncludedHistoryDaysINR {
				h = IncludedHistoryDaysINR
			}
			if h > ExtendedHistoryDays {
				h = ExtendedHistoryDays
			}
			o.HistoryDays = &h
		}
	}
	deal.Notes = strings.TrimSpace(deal.Notes)
	return deal, nil
}

func QuoteFromCustomDeal(deal CustomDeal) SubscriptionQuote {
	monthly := deal.MonthlyPrice
	annual := deal.AnnualPrice
	if annual <= 0 {
		annual = monthly * AnnualMultiplier
	}
	return SubscriptionQuote{
		MonthlySubtotal:         monthly,
		AnnualTotal:             annual,
		AnnualMonthlyEquivalent: int(math.Round(float64(annual) / 12)),
		AnnualSavings:           monthly,
		LineItems: []SubscriptionLineItem{{
			ID:     "custom_deal",
			Label:  "Custom commercial deal",
			Amount: monthly,
		}},
		Selection:       deal.Selection,
		BundledStaff:    IncludedStaffINR + deal.Selection.ExtraStaff,
		BundledChefs:    IncludedChefsINR + deal.Selection.ExtraChefs,
		BundledManagers: IncludedManagersINR + deal.Selection.ExtraManagers,
		TableBundles:    0,
		CityTier:        "",
	}
}

// QuoteFromConfig returns catalog or custom-deal pricing for renewals.
func QuoteFromConfig(cfg StoredSubscriptionConfig, cityTier string) SubscriptionQuote {
	if cfg.IsCustomDeal() {
		return QuoteFromCustomDeal(*cfg.CustomDeal)
	}
	return CalculateSubscriptionQuoteForTier(cfg.Selection, cityTier)
}

func applyLimitsOverride(limits *SubscriptionLimits, o *CustomLimitsOverride) {
	if o == nil {
		return
	}
	if o.MaxTables != nil {
		limits.MaxTables = *o.MaxTables
	}
	if o.MaxStaff != nil {
		limits.MaxStaff = *o.MaxStaff
	}
	if o.MaxChefs != nil {
		limits.MaxChefs = *o.MaxChefs
	}
	if o.MaxManagers != nil {
		limits.MaxManagers = *o.MaxManagers
	}
	limits.MaxStaffAndChefs = limits.MaxStaff + limits.MaxChefs
	if o.HistoryDays != nil {
		limits.HistoryDays = *o.HistoryDays
	}
	if o.Inventory != nil {
		limits.Inventory = *o.Inventory
	}
	if o.Expenses != nil {
		limits.Expenses = *o.Expenses
	}
	if o.KitchenDine != nil {
		limits.KitchenDineIn = *o.KitchenDine
	}
	if o.KitchenCtr != nil {
		limits.KitchenCounter = *o.KitchenCtr
	}
	if o.DineIn != nil {
		limits.DineInEnabled = *o.DineIn
	}
	if o.Counter != nil {
		limits.CounterEnabled = *o.Counter
	}
}

// LimitsFromConfig builds limits from catalog selection or custom deal.
func LimitsFromConfig(cfg StoredSubscriptionConfig, monthlyPriceHint int) SubscriptionLimits {
	if cfg.IsCustomDeal() {
		deal := cfg.CustomDeal
		// Build limits from deal selection without forcing catalog table cap.
		sel := deal.Selection
		maxStaff := IncludedStaffINR + sel.ExtraStaff
		maxChefs := IncludedChefsINR + sel.ExtraChefs
		maxTables := sel.MaxTables
		if maxTables <= 0 {
			maxTables = PlanStarterTables
		}
		limits := SubscriptionLimits{
			OperationMode:    "both",
			MaxTables:        maxTables,
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
			MonthlyPrice:     deal.MonthlyPrice,
		}
		if sel.HistoryExtended {
			limits.HistoryDays = ExtendedHistoryDays
		}
		applyLimitsOverride(&limits, deal.LimitsOverride)
		if monthlyPriceHint > 0 {
			limits.MonthlyPrice = monthlyPriceHint
		} else {
			limits.MonthlyPrice = deal.MonthlyPrice
		}
		return limits
	}
	return LimitsFromSelection(cfg.Selection, monthlyPriceHint)
}

func SelfServePlanChangesLocked(cfg StoredSubscriptionConfig) bool {
	return cfg.IsCustomDeal() && cfg.CustomDeal.LockSelfServeChanges
}

func FormatCustomDealSummary(deal CustomDeal) string {
	return fmt.Sprintf("Custom deal ₹%d/mo (lock=%v)", deal.MonthlyPrice, deal.LockSelfServeChanges)
}
