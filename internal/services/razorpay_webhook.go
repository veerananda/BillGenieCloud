package services

import (
	"encoding/json"
	"strings"
)

// RazorpayWebhookEvent is the envelope Razorpay posts to webhook URLs.
type RazorpayWebhookEvent struct {
	Event   string `json:"event"`
	Payload struct {
		Payment *struct {
			Entity struct {
				ID      string `json:"id"`
				OrderID string `json:"order_id"`
				Status  string `json:"status"`
			} `json:"entity"`
		} `json:"payment"`
		Order *struct {
			Entity struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"entity"`
		} `json:"order"`
	} `json:"payload"`
}

func ParseRazorpayWebhookEvent(body []byte) (*RazorpayWebhookEvent, error) {
	var event RazorpayWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// PaymentReference extracts order and payment IDs from supported subscription events.
func (e *RazorpayWebhookEvent) PaymentReference() (orderID, paymentID string, ok bool) {
	if e == nil {
		return "", "", false
	}

	switch strings.TrimSpace(e.Event) {
	case "payment.captured", "payment.authorized":
		if e.Payload.Payment == nil {
			return "", "", false
		}
		entity := e.Payload.Payment.Entity
		orderID = strings.TrimSpace(entity.OrderID)
		paymentID = strings.TrimSpace(entity.ID)
		if orderID == "" || paymentID == "" {
			return "", "", false
		}
		if e.Event == "payment.authorized" && entity.Status != "authorized" {
			return "", "", false
		}
		if e.Event == "payment.captured" && entity.Status != "captured" {
			return "", "", false
		}
		return orderID, paymentID, true
	case "order.paid":
		if e.Payload.Order == nil {
			return "", "", false
		}
		orderID = strings.TrimSpace(e.Payload.Order.Entity.ID)
		if orderID == "" || e.Payload.Order.Entity.Status != "paid" {
			return "", "", false
		}
		// order.paid may not include payment id; completion uses order lookup only.
		return orderID, "", true
	default:
		return "", "", false
	}
}
