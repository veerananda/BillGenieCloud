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
)

const subscriptionGSTPercent = 18

type SubscriptionRenewalService struct {
	db       *gorm.DB
	razorpay *RazorpayService
}

type RenewalQuote struct {
	BillingCycle    string                 `json:"billing_cycle"`
	SubtotalINR     int                    `json:"subtotal_inr"`
	GSTINR          int                    `json:"gst_inr"`
	TotalINR        int                    `json:"total_inr"`
	AmountPaise     int                    `json:"amount_paise"`
	LineItems       []SubscriptionLineItem `json:"line_items"`
	SubscriptionEnd time.Time              `json:"subscription_end"`
	IsExpired       bool                   `json:"is_expired"`
	DaysRemaining   int                    `json:"days_remaining"`
}

type CreateRenewalOrderResult struct {
	KeyID         string `json:"key_id"`
	OrderID       string `json:"order_id"`
	AmountPaise   int    `json:"amount"`
	Currency      string `json:"currency"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	BillingCycle  string `json:"billing_cycle"`
	TotalINR      int    `json:"total_inr"`
	SubtotalINR   int    `json:"subtotal_inr"`
	GSTINR        int    `json:"gst_inr"`
	DevMode       bool   `json:"dev_mode,omitempty"`
}

type VerifyRenewalPaymentRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
}

type VerifyRenewalPaymentResult struct {
	SubscriptionEnd time.Time `json:"subscription_end"`
	Message         string    `json:"message"`
}

func NewSubscriptionRenewalService(db *gorm.DB) *SubscriptionRenewalService {
	return &SubscriptionRenewalService{
		db:       db,
		razorpay: NewRazorpayService(),
	}
}

func (s *SubscriptionRenewalService) loadRestaurant(restaurantID string) (*models.Restaurant, SubscriptionSelection, SubscriptionQuote, error) {
	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return nil, SubscriptionSelection{}, SubscriptionQuote{}, err
	}

	selection := DefaultSubscriptionSelection()
	if len(restaurant.SubscriptionConfig) > 0 {
		var stored struct {
			Selection SubscriptionSelection `json:"selection"`
		}
		if err := json.Unmarshal(restaurant.SubscriptionConfig, &stored); err == nil {
			if validated, vErr := ValidateSubscriptionSelection(stored.Selection); vErr == nil {
				selection = validated
			}
		}
	}
	quote := CalculateSubscriptionQuote(selection)
	return &restaurant, selection, quote, nil
}

func renewalAmounts(quote SubscriptionQuote, billingCycle string) (subtotal, gst, total, amountPaise int) {
	subtotal = quote.MonthlySubtotal
	if billingCycle == "annual" {
		subtotal = quote.AnnualTotal
	}
	gst = int(math.Round(float64(subtotal) * subscriptionGSTPercent / 100))
	total = subtotal + gst
	amountPaise = total * 100
	return
}

func (s *SubscriptionRenewalService) GetRenewalQuote(restaurantID string) (*RenewalQuote, error) {
	restaurant, selection, quote, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	subtotal, gst, total, amountPaise := renewalAmounts(quote, selection.BillingCycle)
	daysRemaining := int(time.Until(restaurant.SubscriptionEnd).Hours() / 24)

	return &RenewalQuote{
		BillingCycle:    selection.BillingCycle,
		SubtotalINR:     subtotal,
		GSTINR:          gst,
		TotalINR:        total,
		AmountPaise:     amountPaise,
		LineItems:       quote.LineItems,
		SubscriptionEnd: restaurant.SubscriptionEnd,
		IsExpired:       time.Now().After(restaurant.SubscriptionEnd),
		DaysRemaining:   daysRemaining,
	}, nil
}

func (s *SubscriptionRenewalService) CreateRenewalOrder(restaurantID string) (*CreateRenewalOrderResult, error) {
	restaurant, selection, quote, err := s.loadRestaurant(restaurantID)
	if err != nil {
		return nil, err
	}

	subtotal, gst, total, amountPaise := renewalAmounts(quote, selection.BillingCycle)
	billingCycle := selection.BillingCycle
	if billingCycle == "" {
		billingCycle = "monthly"
	}

	periodLabel := "month"
	if billingCycle == "annual" {
		periodLabel = "year"
	}
	description := fmt.Sprintf("BillGenie subscription renewal (%s)", periodLabel)

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

	renewal := models.SubscriptionRenewal{
		RestaurantID:    restaurantID,
		RazorpayOrderID: orderID,
		AmountPaise:     amountPaise,
		BillingCycle:    billingCycle,
		Status:          "pending",
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

	var renewal models.SubscriptionRenewal
	if err := s.db.Where("razorpay_order_id = ? AND restaurant_id = ?", orderID, restaurantID).First(&renewal).Error; err != nil {
		return nil, errors.New("renewal order not found")
	}
	if renewal.Status == "completed" {
		var restaurant models.Restaurant
		if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
			return nil, err
		}
		return &VerifyRenewalPaymentResult{
			SubscriptionEnd: restaurant.SubscriptionEnd,
			Message:         "Subscription already renewed",
		}, nil
	}

	valid := false
	if IsDevMockOrder(orderID) {
		if strings.ToLower(os.Getenv("SERVER_ENV")) == "production" {
			return nil, errors.New("invalid payment")
		}
		// Dev-only: accept mock payments without Razorpay keys.
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

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		return nil, err
	}

	base := time.Now()
	if restaurant.SubscriptionEnd.After(base) {
		base = restaurant.SubscriptionEnd
	}
	if renewal.BillingCycle == "annual" {
		restaurant.SubscriptionEnd = base.AddDate(1, 0, 0)
	} else {
		restaurant.SubscriptionEnd = base.AddDate(0, 1, 0)
	}

	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&restaurant).Error; err != nil {
			return err
		}
		renewal.Status = "completed"
		renewal.PaymentID = paymentID
		renewal.CompletedAt = &now
		return tx.Save(&renewal).Error
	}); err != nil {
		return nil, err
	}

	return &VerifyRenewalPaymentResult{
		SubscriptionEnd: restaurant.SubscriptionEnd,
		Message:         "Subscription renewed successfully",
	}, nil
}
