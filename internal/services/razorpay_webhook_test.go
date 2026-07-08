package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestRazorpayWebhookEventPaymentReference(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantOrder string
		wantPay   string
		wantOK    bool
	}{
		{
			name: "payment captured",
			body: `{
				"event":"payment.captured",
				"payload":{"payment":{"entity":{"id":"pay_123","order_id":"order_abc","status":"captured"}}}
			}`,
			wantOrder: "order_abc",
			wantPay:   "pay_123",
			wantOK:    true,
		},
		{
			name: "payment authorized",
			body: `{
				"event":"payment.authorized",
				"payload":{"payment":{"entity":{"id":"pay_456","order_id":"order_def","status":"authorized"}}}
			}`,
			wantOrder: "order_def",
			wantPay:   "pay_456",
			wantOK:    true,
		},
		{
			name: "order paid without payment id",
			body: `{
				"event":"order.paid",
				"payload":{"order":{"entity":{"id":"order_xyz","status":"paid"}}}
			}`,
			wantOrder: "order_xyz",
			wantPay:   "",
			wantOK:    true,
		},
		{
			name:   "unsupported event",
			body:   `{"event":"refund.created"}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseRazorpayWebhookEvent([]byte(tt.body))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			orderID, paymentID, ok := event.PaymentReference()
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if orderID != tt.wantOrder {
				t.Fatalf("orderID = %q, want %q", orderID, tt.wantOrder)
			}
			if paymentID != tt.wantPay {
				t.Fatalf("paymentID = %q, want %q", paymentID, tt.wantPay)
			}
		})
	}
}

func TestRazorpayVerifyWebhookSignature(t *testing.T) {
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "whsec_test")

	body := []byte(`{"event":"payment.captured"}`)
	mac := hmac.New(sha256.New, []byte("whsec_test"))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	svc := NewRazorpayService()
	if !svc.VerifyWebhookSignature(body, signature) {
		t.Fatal("expected valid webhook signature")
	}
	if svc.VerifyWebhookSignature(body, "bad-signature") {
		t.Fatal("expected invalid webhook signature")
	}
}
