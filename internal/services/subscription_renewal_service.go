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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const subscriptionGSTPercent = 18

type SubscriptionRenewalService struct {
	db           *gorm.DB
	razorpay     *RazorpayService
	trialService *TrialEligibilityService
}

type RenewalQuote struct {
	BillingCycle          string                 `json:"billing_cycle"`
	SubtotalINR           int                    `json:"subtotal_inr"`
	GSTINR                int                    `json:"gst_inr"`
	TotalINR              int                    `json:"total_inr"`
	AmountPaise           int                    `json:"amount_paise"`
	LineItems             []SubscriptionLineItem `json:"line_items"`
	SubscriptionEnd       time.Time              `json:"subscription_end"`
	IsExpired             bool                   `json:"is_expired"`
	DaysRemaining         int                    `json:"days_remaining"`
	SubscriptionPhase     string                 `json:"subscription_phase"`
	RequiresPlanSelection bool                   `json:"requires_plan_selection"`
	RequiresPayment       bool                   `json:"requires_payment"`
	CurrentSelection      *SubscriptionSelection `json:"current_selection,omitempty"`
}

type CreateRenewalOrderResult struct {
	KeyID        string `json:"key_id"`
	OrderID      string `json:"order_id"`
	AmountPaise  int    `json:"amount"`
	Currency     string `json:"currency"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	BillingCycle string `json:"billing_cycle"`
	TotalINR     int    `json:"total_inr"`
	SubtotalINR  int    `json:"subtotal_inr"`
	GSTINR       int    `json:"gst_inr"`
	DevMode      bool   `json:"dev_mode,omitempty"`
}

type VerifyRenewalPaymentRequest struct {
	RazorpayOrderID   string                `json:"razorpay_order_id"`
	RazorpayPaymentID string                `json:"razorpay_payment_id"`
	RazorpaySignature string                `json:"razorpay_signature"`
	Selection         *SubscriptionSelection `json:"selection,omitempty"`
}

type VerifyRenewalPaymentResult struct {
	SubscriptionEnd time.Time `json:"subscription_end"`
	Message         string    `json:"message"`
}

func NewSubscriptionRenewalService(db *gorm.DB) *SubscriptionRenewalService {
	return &SubscriptionRenewalService{
		db:           db,
		razorpay:     NewRazorpayService(),
		trialService: NewTrialEligibilityService(db),
	}
}

func (s *SubscriptionRenewalService) loadRestaurant(restaurantID string) (*models.Restaurant, StoredSubscriptionConfig, SubscriptionSelection, SubscriptionQuote, error) {
	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return nil, StoredSubscriptionConfig{}, SubscriptionSelection{}, SubscriptionQuote{}, err
	}

	cfg := ParseStoredSubscriptionConfig(&restaurant)
	selection := cfg.Selection
	quote := CalculateSubscriptionQuote(selection)
	if cfg.Quote.MonthlySubtotal > 0 {
		quote = cfg.Quote
	}
	return &restaurant, cfg, selection, quote, nil
}

func quoteAmounts(quote SubscriptionQuote, billingCycle string) (subtotal, gst, total, amountPaise int) {
	subtotal = quote.MonthlySubtotal
	if billingCycle == "annual" {
		subtotal = quote.AnnualTotal
	}
	gst = int(math.Round(float64(subtotal) * subscriptionGSTPercent / 100))
	total = subtotal + gst
	amountPaise = total * 100
	return
}

func (s *SubscriptionRenewalService) QuoteForSelection(selection SubscriptionSelection) (*RenewalQuote, error) {
	validated, err := ValidateSubscriptionSelection(selection)
	if err != nil {
		return nil, err
	}
	quote := CalculateSubscriptionQuote(validated)
	subtotal, gst, total, amountPaise := quoteAmounts(quote, validated.BillingCycle)
	return &RenewalQuote{
		BillingCycle:    validated.BillingCycle,
		SubtotalINR:     subtotal,
		GSTINR:          gst,
		TotalINR:        total,
		AmountPaise:     amountPaise,
		LineItems:       quote.LineItems,
		RequiresPayment: true,
	}, nil
}

func (s *SubscriptionRenewalService) GetRenewalQuote(restaurantID string, selectionOverride *SubscriptionSelection) (*RenewalQuote, error) {
	restaurant, cfg, selection, quote, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	requiresPlan := AllowsPlanReview(restaurant)
	requiresPayment := cfg.Phase == SubscriptionPhasePendingPayment || IsSubscriptionAccessBlocked(restaurant)

	if selectionOverride != nil {
		validated, err := ValidateSubscriptionSelection(*selectionOverride)
		if err != nil {
			return nil, err
		}
		selection = validated
		quote = CalculateSubscriptionQuote(selection)
	}

	billingCycle := selection.BillingCycle
	if billingCycle == "" {
		billingCycle = "monthly"
	}
	subtotal, gst, total, amountPaise := quoteAmounts(quote, billingCycle)
	daysRemaining := int(time.Until(restaurant.SubscriptionEnd).Hours() / 24)
	currentSelection := selection

	return &RenewalQuote{
		BillingCycle:          billingCycle,
		SubtotalINR:           subtotal,
		GSTINR:                gst,
		TotalINR:              total,
		AmountPaise:           amountPaise,
		LineItems:             quote.LineItems,
		SubscriptionEnd:       restaurant.SubscriptionEnd,
		IsExpired:             IsSubscriptionAccessBlocked(restaurant),
		DaysRemaining:         daysRemaining,
		SubscriptionPhase:     cfg.Phase,
		RequiresPlanSelection: requiresPlan,
		RequiresPayment:       requiresPayment,
		CurrentSelection:      &currentSelection,
	}, nil
}

func (s *SubscriptionRenewalService) CreateRenewalOrder(restaurantID string, selectionOverride *SubscriptionSelection) (*CreateRenewalOrderResult, error) {
	restaurant, cfg, selection, quote, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	requiresPlan := AllowsPlanReview(restaurant)
	if requiresPlan && selectionOverride == nil && cfg.Phase != SubscriptionPhasePendingPayment {
		return nil, errors.New("choose a subscription plan before payment")
	}

	if selectionOverride != nil {
		validated, err := ValidateSubscriptionSelection(*selectionOverride)
		if err != nil {
			return nil, err
		}
		selection = validated
		quote = CalculateSubscriptionQuote(selection)
	}

	billingCycle := selection.BillingCycle
	if billingCycle == "" {
		billingCycle = "monthly"
	}
	subtotal, gst, total, amountPaise := quoteAmounts(quote, billingCycle)

	periodLabel := "month"
	if billingCycle == "annual" {
		periodLabel = "year"
	}
	description := fmt.Sprintf("BillGenie subscription (%s)", periodLabel)
	if cfg.Phase == SubscriptionPhasePendingPayment {
		description = "BillGenie subscription activation"
	} else if requiresPlan {
		description = "BillGenie plan selection"
	}

	var orderID string
	devMode := false

	if s.razorpay.IsConfigured() {
		receipt := fmt.Sprintf("renew_%s_%d", restaurantID[:8], time.Now().Unix())
		order, err := s.razorpay.CreateOrder(amountPaise, receipt, map[string]string{
			"restaurant_id": restaurantID,
			"billing_cycle": billingCycle,
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

	pendingJSON, _ := json.Marshal(selection)
	renewal := models.SubscriptionRenewal{
		RestaurantID:     restaurantID,
		RazorpayOrderID:  orderID,
		AmountPaise:      amountPaise,
		BillingCycle:     billingCycle,
		Status:           "pending",
		PendingSelection: pendingJSON,
	}
	if err := s.db.Create(&renewal).Error; err != nil {
		return nil, err
	}

	return &CreateRenewalOrderResult{
		KeyID:        s.razorpay.KeyID(),
		OrderID:      orderID,
		AmountPaise:  amountPaise,
		Currency:     "INR",
		Name:         restaurant.Name,
		Description:  description,
		BillingCycle: billingCycle,
		TotalINR:     total,
		SubtotalINR:  subtotal,
		GSTINR:       gst,
		DevMode:      devMode,
	}, nil
}

func (s *SubscriptionRenewalService) applyPaidSelection(restaurant *models.Restaurant, cfg StoredSubscriptionConfig, selection SubscriptionSelection, billingCycle string) error {
	validated, err := ValidateSubscriptionSelection(selection)
	if err != nil {
		return err
	}
	quote := CalculateSubscriptionQuote(validated)

	counterModes := "both"
	isSelfService := false
	ApplyOperationModeToRestaurant(&isSelfService, &counterModes, validated.OperationMode)

	restaurant.IsSelfService = isSelfService
	restaurant.CounterServiceModes = counterModes
	restaurant.SubscriptionMonthlyPrice = quote.MonthlySubtotal
	restaurant.SubscriptionPlan = SubscriptionPlanFromSelection(validated)

	startMode := cfg.StartMode
	if startMode == "" {
		startMode = "paid"
	}
	configJSON, err := BuildSubscriptionConfigJSON(SubscriptionPhaseActive, startMode, validated, quote, true)
	if err != nil {
		return err
	}
	restaurant.SubscriptionConfig = configJSON

	restaurant.SubscriptionEnd = NextSubscriptionEnd(restaurant.SubscriptionEnd, billingCycle)
	return nil
}

// NextSubscriptionEnd returns the new paid period end.
// If the current subscription is still active, unused days are preserved
// (extend from subscription_end). If already expired (or never set), the
// period starts from payment/activation time.
func NextSubscriptionEnd(currentEnd time.Time, billingCycle string) time.Time {
	base := time.Now()
	if !currentEnd.IsZero() && currentEnd.After(base) {
		base = currentEnd
	}
	if strings.EqualFold(strings.TrimSpace(billingCycle), "annual") {
		return base.AddDate(1, 0, 0)
	}
	return base.AddDate(0, 1, 0)
}

func (s *SubscriptionRenewalService) VerifyRenewalPayment(restaurantID string, req VerifyRenewalPaymentRequest) (*VerifyRenewalPaymentResult, error) {
	orderID := strings.TrimSpace(req.RazorpayOrderID)
	paymentID := strings.TrimSpace(req.RazorpayPaymentID)
	signature := strings.TrimSpace(req.RazorpaySignature)
	if orderID == "" || paymentID == "" {
		return nil, errors.New("missing payment verification fields")
	}
	if !IsDevMockOrder(orderID) && signature == "" {
		return nil, errors.New("missing payment verification fields")
	}

	valid := false
	if IsDevMockOrder(orderID) {
		if strings.ToLower(os.Getenv("SERVER_ENV")) == "production" {
			return nil, errors.New("invalid payment")
		}
		if strings.HasPrefix(paymentID, "pay_dev_") {
			valid = true
		} else {
			valid = VerifyDevMockSignature(orderID, paymentID, signature)
		}
	} else {
		valid = s.razorpay.VerifyPaymentSignature(orderID, paymentID, signature)
	}
	if !valid {
		return nil, errors.New("payment verification failed")
	}

	return s.completeRenewalPayment(restaurantID, orderID, paymentID, req.Selection)
}

// ProcessRazorpayWebhook activates subscription when Razorpay notifies payment success.
func (s *SubscriptionRenewalService) ProcessRazorpayWebhook(body []byte) error {
	event, err := ParseRazorpayWebhookEvent(body)
	if err != nil {
		return err
	}

	orderID, paymentID, ok := event.PaymentReference()
	if !ok {
		return nil
	}
	if paymentID == "" {
		return nil
	}

	_, err = s.completeRenewalPayment("", orderID, paymentID, nil)
	if err == nil {
		return nil
	}
	if errors.Is(err, errRenewalOrderNotFound) || errors.Is(err, errRenewalOrderStale) {
		return nil
	}
	return err
}

var (
	errRenewalOrderNotFound = errors.New("renewal order not found")
	errRenewalOrderStale    = errors.New("renewal order is stale")
)

func (s *SubscriptionRenewalService) completeRenewalPayment(
	restaurantID string,
	orderID string,
	paymentID string,
	selectionOverride *SubscriptionSelection,
) (*VerifyRenewalPaymentResult, error) {
	orderID = strings.TrimSpace(orderID)
	paymentID = strings.TrimSpace(paymentID)
	if orderID == "" || paymentID == "" {
		return nil, errors.New("missing payment verification fields")
	}

	var result VerifyRenewalPaymentResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var renewal models.SubscriptionRenewal
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("razorpay_order_id = ?", orderID)
		if strings.TrimSpace(restaurantID) != "" {
			query = query.Where("restaurant_id = ?", restaurantID)
		}
		if err := query.First(&renewal).Error; err != nil {
			return errRenewalOrderNotFound
		}

		resolvedRestaurantID := renewal.RestaurantID
		if strings.TrimSpace(restaurantID) != "" && resolvedRestaurantID != restaurantID {
			return errRenewalOrderNotFound
		}

		var restaurant models.Restaurant
		if err := tx.Where("id = ?", resolvedRestaurantID).First(&restaurant).Error; err != nil {
			return err
		}

		if renewal.Status == "completed" {
			result = VerifyRenewalPaymentResult{
				SubscriptionEnd: restaurant.SubscriptionEnd,
				Message:         "Subscription already renewed",
			}
			return nil
		}
		if renewal.Status != "pending" {
			return fmt.Errorf("%w: %s", errRenewalOrderStale, renewal.Status)
		}

		cfg := ParseStoredSubscriptionConfig(&restaurant)
		selection := cfg.Selection
		if len(renewal.PendingSelection) > 0 {
			if err := json.Unmarshal(renewal.PendingSelection, &selection); err != nil {
				return err
			}
		}
		if selectionOverride != nil {
			validated, err := ValidateSubscriptionSelection(*selectionOverride)
			if err != nil {
				return err
			}
			selection = validated
		}
		if NeedsPlanSelection(&restaurant) && selectionOverride == nil && len(renewal.PendingSelection) == 0 {
			return errors.New("subscription plan selection is required")
		}

		if err := s.applyPaidSelection(&restaurant, cfg, selection, renewal.BillingCycle); err != nil {
			return err
		}

		now := time.Now()
		renewalUpdate := tx.Model(&models.SubscriptionRenewal{}).
			Where("id = ? AND status = ?", renewal.ID, "pending").
			Updates(map[string]interface{}{
				"status":       "completed",
				"payment_id":   paymentID,
				"completed_at": now,
			})
		if renewalUpdate.Error != nil {
			return renewalUpdate.Error
		}
		if renewalUpdate.RowsAffected == 0 {
			if err := tx.Where("id = ?", resolvedRestaurantID).First(&restaurant).Error; err != nil {
				return err
			}
			result = VerifyRenewalPaymentResult{
				SubscriptionEnd: restaurant.SubscriptionEnd,
				Message:         "Subscription already renewed",
			}
			return nil
		}

		if err := tx.Save(&restaurant).Error; err != nil {
			return err
		}

		if err := s.trialService.MarkConverted(resolvedRestaurantID); err != nil {
			return err
		}

		message := "Subscription activated successfully"
		if cfg.Phase == SubscriptionPhaseActive {
			message = "Subscription renewed successfully"
		}
		result = VerifyRenewalPaymentResult{
			SubscriptionEnd: restaurant.SubscriptionEnd,
			Message:         message,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
