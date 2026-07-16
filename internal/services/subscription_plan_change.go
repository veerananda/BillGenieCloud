package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
)

const (
	PlanChangeUpgrade   = "upgrade"
	PlanChangeDowngrade = "downgrade"
	PlanChangeNoop      = "noop"
	RenewalKindRenew    = "renew"
	RenewalKindUpgrade  = "upgrade"
)

type PlanChangeQuote struct {
	ChangeType              string                `json:"change_type"`
	BillingCycle            string                `json:"billing_cycle"`
	RemainingDays           int                   `json:"remaining_days"`
	PeriodDays              int                   `json:"period_days"`
	CurrentSelection        SubscriptionSelection `json:"current_selection"`
	NewSelection            SubscriptionSelection `json:"new_selection"`
	CurrentPeriodAmountINR  int                   `json:"current_period_amount_inr"`
	NewPeriodAmountINR      int                   `json:"new_period_amount_inr"`
	ProrationDeltaINR       int                   `json:"proration_delta_inr"`
	NextPeriodAmountINR     int                   `json:"next_period_amount_inr"`
	AmountDueINR            int                   `json:"amount_due_inr"`
	AmountPaise             int                   `json:"amount_paise"`
	GSTINR                  int                   `json:"gst_inr"`
	SubtotalINR             int                   `json:"subtotal_inr"`
	LineItems               []SubscriptionLineItem `json:"line_items"`
	EffectiveAt             string                `json:"effective_at"` // immediate | ISO date
	NewSubscriptionEnd      time.Time             `json:"new_subscription_end"`
	CurrentSubscriptionEnd  time.Time             `json:"current_subscription_end"`
	PendingChangeAt         *time.Time            `json:"pending_change_at,omitempty"`
	HasPendingDowngrade     bool                  `json:"has_pending_downgrade"`
}

type SchedulePlanChangeResult struct {
	Message         string                `json:"message"`
	PendingSelection SubscriptionSelection `json:"pending_selection"`
	PendingChangeAt time.Time             `json:"pending_change_at"`
	SubscriptionEnd time.Time             `json:"subscription_end"`
}

func periodAmountINR(quote SubscriptionQuote, billingCycle string) (subtotal, gst, total int) {
	subtotal, gst, total, _ = quoteAmounts(quote, billingCycle)
	return
}

func subscriptionPeriodDays(cfg StoredSubscriptionConfig, subscriptionEnd time.Time, billingCycle string) int {
	now := time.Now()
	fallback := 30
	if strings.EqualFold(billingCycle, "annual") {
		fallback = 365
	}
	if cfg.PeriodStartedAt != nil && !cfg.PeriodStartedAt.IsZero() && subscriptionEnd.After(*cfg.PeriodStartedAt) {
		days := int(math.Ceil(subscriptionEnd.Sub(*cfg.PeriodStartedAt).Hours() / 24))
		if days < 1 {
			return fallback
		}
		return days
	}
	// Infer: remaining + assume started (period - remaining) ago is unknown; use fallback.
	_ = now
	return fallback
}

func remainingSubscriptionDays(subscriptionEnd time.Time) int {
	if subscriptionEnd.IsZero() || !subscriptionEnd.After(time.Now()) {
		return 0
	}
	days := int(math.Ceil(time.Until(subscriptionEnd).Hours() / 24))
	if days < 1 {
		return 1
	}
	return days
}

// QuotePlanChange computes upgrade (proration delta + next period) or downgrade (schedule) quote.
func (s *SubscriptionRenewalService) QuotePlanChange(restaurantID string, newSel SubscriptionSelection) (*PlanChangeQuote, error) {
	restaurant, cfg, currentSel, _, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}
	if !CanChangePlanMidCycle(restaurant) {
		return nil, errors.New("plan changes are only available for an active paid subscription; renew instead")
	}

	validated, err := ValidateSubscriptionSelection(newSel)
	if err != nil {
		return nil, err
	}
	if validated.BillingCycle == "" {
		validated.BillingCycle = currentSel.BillingCycle
	}
	if validated.BillingCycle != currentSel.BillingCycle {
		return nil, errors.New("billing cycle cannot be changed mid-cycle; change cycle at renewal")
	}

	oldQuote := CalculateSubscriptionQuote(currentSel)
	newQuote := CalculateSubscriptionQuote(validated)
	_, _, oldPeriod := periodAmountINR(oldQuote, currentSel.BillingCycle)
	_, _, newPeriod := periodAmountINR(newQuote, validated.BillingCycle)

	remaining := remainingSubscriptionDays(restaurant.SubscriptionEnd)
	periodDays := subscriptionPeriodDays(cfg, restaurant.SubscriptionEnd, currentSel.BillingCycle)
	if periodDays < remaining {
		periodDays = remaining
	}
	if periodDays < 1 {
		periodDays = 1
	}

	prorationDelta := int(math.Round(float64(newPeriod-oldPeriod) * float64(remaining) / float64(periodDays)))
	if prorationDelta < 0 {
		prorationDelta = 0
	}

	changeType := PlanChangeNoop
	if selectionEqual(currentSel, validated) {
		changeType = PlanChangeNoop
	} else if newPeriod > oldPeriod {
		changeType = PlanChangeUpgrade
	} else {
		changeType = PlanChangeDowngrade
		prorationDelta = 0
	}

	amountDue := 0
	effectiveAt := "immediate"
	newEnd := restaurant.SubscriptionEnd
	if changeType == PlanChangeUpgrade {
		amountDue = prorationDelta + newPeriod
		newEnd = NextSubscriptionEnd(restaurant.SubscriptionEnd, validated.BillingCycle)
	} else if changeType == PlanChangeDowngrade {
		effectiveAt = restaurant.SubscriptionEnd.UTC().Format(time.RFC3339)
		amountDue = 0
	}

	subtotalDue := 0
	gstDue := 0
	if amountDue > 0 {
		subtotalDue = int(math.Round(float64(amountDue) * 100 / float64(100+subscriptionGSTPercent)))
		gstDue = amountDue - subtotalDue
	}

	out := &PlanChangeQuote{
		ChangeType:             changeType,
		BillingCycle:           validated.BillingCycle,
		RemainingDays:          remaining,
		PeriodDays:             periodDays,
		CurrentSelection:       currentSel,
		NewSelection:           validated,
		CurrentPeriodAmountINR: oldPeriod,
		NewPeriodAmountINR:     newPeriod,
		ProrationDeltaINR:      prorationDelta,
		NextPeriodAmountINR:    newPeriod,
		AmountDueINR:           amountDue,
		AmountPaise:            amountDue * 100,
		GSTINR:                 gstDue,
		SubtotalINR:            subtotalDue,
		LineItems:              newQuote.LineItems,
		EffectiveAt:            effectiveAt,
		NewSubscriptionEnd:     newEnd,
		CurrentSubscriptionEnd: restaurant.SubscriptionEnd,
		HasPendingDowngrade:    cfg.PendingSelection != nil,
	}
	if cfg.PendingChangeAt != nil {
		out.PendingChangeAt = cfg.PendingChangeAt
	}
	return out, nil
}

func selectionEqual(a, b SubscriptionSelection) bool {
	return a.BillingCycle == b.BillingCycle &&
		a.OperationMode == b.OperationMode &&
		a.MaxTables == b.MaxTables &&
		a.ExtraStaff == b.ExtraStaff &&
		a.ExtraManagers == b.ExtraManagers &&
		a.HistoryExtended == b.HistoryExtended &&
		a.Inventory == b.Inventory &&
		a.KitchenDineIn == b.KitchenDineIn &&
		a.KitchenCounter == b.KitchenCounter
}

func (s *SubscriptionRenewalService) CreatePlanChangeOrder(restaurantID string, newSel SubscriptionSelection) (*CreateRenewalOrderResult, error) {
	quote, err := s.QuotePlanChange(restaurantID, newSel)
	if err != nil {
		return nil, err
	}
	if quote.ChangeType != PlanChangeUpgrade {
		return nil, errors.New("only upgrades require payment; schedule a downgrade instead")
	}
	if quote.AmountDueINR <= 0 || quote.AmountPaise <= 0 {
		return nil, errors.New("nothing to charge for this upgrade")
	}

	restaurant, _, _, _, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	var orderID string
	devMode := false
	if s.razorpay.IsConfigured() {
		receipt := fmt.Sprintf("upg_%s_%d", restaurantID[:8], time.Now().Unix())
		order, err := s.razorpay.CreateOrder(quote.AmountPaise, receipt, map[string]string{
			"restaurant_id": restaurantID,
			"billing_cycle": quote.BillingCycle,
			"kind":          RenewalKindUpgrade,
		})
		if err != nil {
			return nil, err
		}
		orderID = order.ID
	} else if strings.ToLower(os.Getenv("SERVER_ENV")) != "production" {
		devMode = true
		orderID = DevMockOrderIDPrefix + uuid.New().String()
	} else {
		return nil, errors.New("payment gateway not configured")
	}

	if err := s.db.Model(&models.SubscriptionRenewal{}).
		Where("restaurant_id = ? AND status = ?", restaurantID, "pending").
		Update("status", "superseded").Error; err != nil {
		return nil, err
	}

	pendingJSON, _ := json.Marshal(quote.NewSelection)
	renewal := models.SubscriptionRenewal{
		RestaurantID:     restaurantID,
		RazorpayOrderID:  orderID,
		AmountPaise:      quote.AmountPaise,
		BillingCycle:     quote.BillingCycle,
		Kind:             RenewalKindUpgrade,
		Status:           "pending",
		PendingSelection: pendingJSON,
	}
	if err := s.db.Create(&renewal).Error; err != nil {
		return nil, err
	}

	return &CreateRenewalOrderResult{
		KeyID:        s.razorpay.KeyID(),
		OrderID:      orderID,
		AmountPaise:  quote.AmountPaise,
		Currency:     "INR",
		Name:         restaurant.Name,
		Description:  "BillGenie plan upgrade",
		BillingCycle: quote.BillingCycle,
		TotalINR:     quote.AmountDueINR,
		SubtotalINR:  quote.SubtotalINR,
		GSTINR:       quote.GSTINR,
		DevMode:      devMode,
	}, nil
}

func (s *SubscriptionRenewalService) SchedulePlanChange(restaurantID string, newSel SubscriptionSelection) (*SchedulePlanChangeResult, error) {
	quote, err := s.QuotePlanChange(restaurantID, newSel)
	if err != nil {
		return nil, err
	}
	if quote.ChangeType == PlanChangeUpgrade {
		return nil, errors.New("this change is an upgrade — pay via change-order instead")
	}
	if quote.ChangeType == PlanChangeNoop {
		return nil, errors.New("selection is unchanged")
	}

	restaurant, cfg, _, _, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	newLimits := LimitsFromSelection(quote.NewSelection, quote.NewPeriodAmountINR)
	usage, err := LoadSubscriptionUsage(s.db, restaurantID)
	if err != nil {
		return nil, err
	}
	if err := UsageExceedsLimits(usage, newLimits); err != nil {
		return nil, err
	}

	changeAt := restaurant.SubscriptionEnd
	cfg.PendingSelection = &quote.NewSelection
	cfg.PendingChangeAt = &changeAt
	configJSON, err := MarshalSubscriptionConfig(cfg)
	if err != nil {
		return nil, err
	}
	restaurant.SubscriptionConfig = configJSON
	if err := s.db.Save(restaurant).Error; err != nil {
		return nil, err
	}

	return &SchedulePlanChangeResult{
		Message:          "Downgrade scheduled for the end of your current period",
		PendingSelection: quote.NewSelection,
		PendingChangeAt:  changeAt,
		SubscriptionEnd:  restaurant.SubscriptionEnd,
	}, nil
}

func (s *SubscriptionRenewalService) CancelScheduledPlanChange(restaurantID string) error {
	restaurant, cfg, _, _, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return err
	}
	if cfg.PendingSelection == nil {
		return errors.New("no scheduled plan change to cancel")
	}
	cfg.PendingSelection = nil
	cfg.PendingChangeAt = nil
	configJSON, err := MarshalSubscriptionConfig(cfg)
	if err != nil {
		return err
	}
	restaurant.SubscriptionConfig = configJSON
	return s.db.Save(restaurant).Error
}
