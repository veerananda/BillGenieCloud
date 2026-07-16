package services

import (
	"encoding/json"
	"time"

	"restaurant-api/internal/models"
)

const (
	SubscriptionPhaseTrial          = "trial"
	SubscriptionPhasePendingPayment = "pending_payment"
	SubscriptionPhaseActive         = "active"
	TrialDurationDays               = 15
)

// Fixed trial bundle — keep in sync with BillGenieApp-new/src/config/subscriptionPricing.ts
func FixedTrialSelection() SubscriptionSelection {
	return SubscriptionSelection{
		BillingCycle:    "monthly",
		OperationMode:   "both",
		MaxTables:       IncludedTablesBasic,
		ExtraStaff:      0,
		ExtraManagers:   0,
		HistoryExtended: false,
		Inventory:       false,
		KitchenDineIn:   true,
		KitchenCounter:  true,
	}
}

func TrialSubscriptionLimits() SubscriptionLimits {
	return SubscriptionLimits{
		OperationMode:    "both",
		MaxTables:        IncludedTablesBasic,
		MaxManagers:      1,
		MaxStaffAndChefs: 3,
		MaxChefs:         1,
		HistoryDays:      IncludedHistoryDaysINR,
		KitchenDineIn:    true,
		KitchenCounter:   true,
		Inventory:        false,
		DineInEnabled:    true,
		CounterEnabled:   true,
	}
}

type StoredSubscriptionConfig struct {
	Phase            string                 `json:"phase"`
	Selection        SubscriptionSelection  `json:"selection"`
	Quote            SubscriptionQuote      `json:"quote"`
	HasEverPaid      bool                   `json:"has_ever_paid"`
	StartMode        string                 `json:"start_mode,omitempty"`
	PendingSelection *SubscriptionSelection `json:"pending_selection,omitempty"`
	PendingChangeAt  *time.Time             `json:"pending_change_at,omitempty"`
	PeriodStartedAt  *time.Time             `json:"period_started_at,omitempty"`
}

func ParseStoredSubscriptionConfig(restaurant *models.Restaurant) StoredSubscriptionConfig {
	cfg := StoredSubscriptionConfig{
		Phase:       SubscriptionPhaseTrial,
		Selection:   DefaultSubscriptionSelection(),
		HasEverPaid: false,
	}
	if restaurant == nil || len(restaurant.SubscriptionConfig) == 0 {
		return cfg
	}
	if err := json.Unmarshal(restaurant.SubscriptionConfig, &cfg); err != nil {
		return cfg
	}
	if validated, err := ValidateSubscriptionSelection(cfg.Selection); err == nil {
		cfg.Selection = validated
	}
	if cfg.PendingSelection != nil {
		if validated, err := ValidateSubscriptionSelection(*cfg.PendingSelection); err == nil {
			cfg.PendingSelection = &validated
		} else {
			cfg.PendingSelection = nil
			cfg.PendingChangeAt = nil
		}
	}
	if cfg.Phase == "" {
		cfg.Phase = SubscriptionPhaseTrial
	}
	return cfg
}

func MarshalSubscriptionConfig(cfg StoredSubscriptionConfig) (json.RawMessage, error) {
	bytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(bytes), nil
}

func BuildSubscriptionConfigJSON(phase string, startMode string, sel SubscriptionSelection, quote SubscriptionQuote, hasEverPaid bool) (json.RawMessage, error) {
	payload := StoredSubscriptionConfig{
		Phase:       phase,
		StartMode:   startMode,
		Selection:   sel,
		Quote:       quote,
		HasEverPaid: hasEverPaid,
	}
	return MarshalSubscriptionConfig(payload)
}

func IsSubscriptionAccessBlocked(restaurant *models.Restaurant) bool {
	if restaurant == nil {
		return true
	}
	cfg := ParseStoredSubscriptionConfig(restaurant)
	if cfg.Phase == SubscriptionPhasePendingPayment {
		return true
	}
	return time.Now().After(restaurant.SubscriptionEnd)
}

func NeedsPlanSelection(restaurant *models.Restaurant) bool {
	cfg := ParseStoredSubscriptionConfig(restaurant)
	if cfg.HasEverPaid {
		return false
	}
	if cfg.Phase == SubscriptionPhasePendingPayment {
		return false
	}
	return cfg.Phase == SubscriptionPhaseTrial && time.Now().After(restaurant.SubscriptionEnd)
}

// AllowsPlanReview is true when the customer may review or edit plan details at checkout
// (first paid activation after signup, or post-trial plan selection).
func AllowsPlanReview(restaurant *models.Restaurant) bool {
	cfg := ParseStoredSubscriptionConfig(restaurant)
	if cfg.Phase == SubscriptionPhasePendingPayment {
		return true
	}
	return NeedsPlanSelection(restaurant)
}

// CanChangePlanMidCycle is true for paid active subscriptions that are not expired.
func CanChangePlanMidCycle(restaurant *models.Restaurant) bool {
	if restaurant == nil {
		return false
	}
	cfg := ParseStoredSubscriptionConfig(restaurant)
	if cfg.Phase != SubscriptionPhaseActive || !cfg.HasEverPaid {
		return false
	}
	return time.Now().Before(restaurant.SubscriptionEnd)
}
