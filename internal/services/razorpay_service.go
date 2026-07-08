package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type RazorpayService struct {
	keyID     string
	keySecret string
	client    *http.Client
}

type razorpayOrderRequest struct {
	Amount   int               `json:"amount"`
	Currency string            `json:"currency"`
	Receipt  string            `json:"receipt"`
	Notes    map[string]string `json:"notes,omitempty"`
}

type razorpayOrderResponse struct {
	ID       string `json:"id"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
	Error    *struct {
		Description string `json:"description"`
	} `json:"error"`
}

func NewRazorpayService() *RazorpayService {
	return &RazorpayService{
		keyID:     strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")),
		keySecret: strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET")),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *RazorpayService) IsConfigured() bool {
	return r.keyID != "" && r.keySecret != ""
}

func (r *RazorpayService) KeyID() string {
	return r.keyID
}

func (r *RazorpayService) CreateOrder(amountPaise int, receipt string, notes map[string]string) (*razorpayOrderResponse, error) {
	if !r.IsConfigured() {
		return nil, errors.New("payment gateway not configured")
	}

	payload := razorpayOrderRequest{
		Amount:   amountPaise,
		Currency: "INR",
		Receipt:  receipt,
		Notes:    notes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(r.keyID, r.keySecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var order razorpayOrderResponse
	if err := json.Unmarshal(respBody, &order); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := "failed to create payment order"
		if order.Error != nil && order.Error.Description != "" {
			msg = order.Error.Description
		}
		return nil, fmt.Errorf("%s (status %d)", msg, resp.StatusCode)
	}
	if order.ID == "" {
		return nil, errors.New("razorpay returned empty order id")
	}
	return &order, nil
}

func (r *RazorpayService) VerifyPaymentSignature(orderID, paymentID, signature string) bool {
	if !r.IsConfigured() || orderID == "" || paymentID == "" || signature == "" {
		return false
	}
	payload := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(r.keySecret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (r *RazorpayService) WebhookSecret() string {
	return strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
}

// VerifyWebhookSignature validates Razorpay webhook callbacks (X-Razorpay-Signature).
func (r *RazorpayService) VerifyWebhookSignature(body []byte, signature string) bool {
	secret := r.WebhookSecret()
	if secret == "" || signature == "" || len(body) == 0 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// DevMockOrderID prefix for non-production testing without Razorpay keys.
const DevMockOrderIDPrefix = "order_dev_"

func IsDevMockOrder(orderID string) bool {
	return strings.HasPrefix(orderID, DevMockOrderIDPrefix)
}

func DevMockSignature(orderID, paymentID string) string {
	raw := orderID + "|" + paymentID + "|dev"
	sum := sha256.Sum256([]byte(raw))
	return base64.StdEncoding.EncodeToString(sum[:16])
}

func VerifyDevMockSignature(orderID, paymentID, signature string) bool {
	if !IsDevMockOrder(orderID) {
		return false
	}
	return DevMockSignature(orderID, paymentID) == signature
}
