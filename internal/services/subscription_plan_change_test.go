package services

import (
	"testing"
	"time"
)

func TestRemainingSubscriptionDays(t *testing.T) {
	if remainingSubscriptionDays(time.Now().Add(-time.Hour)) != 0 {
		t.Fatal("expected 0 for past end")
	}
	got := remainingSubscriptionDays(time.Now().Add(48 * time.Hour))
	if got < 2 || got > 3 {
		t.Fatalf("expected ~2 days remaining, got %d", got)
	}
}

func TestPeriodAmountUsesAnnual(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	sel.BillingCycle = "annual"
	sel.Inventory = true
	q := CalculateSubscriptionQuote(sel)
	_, _, monthlyTotal := periodAmountINR(q, "monthly")
	_, _, annualTotal := periodAmountINR(q, "annual")
	if annualTotal <= monthlyTotal {
		t.Fatalf("annual period amount should exceed monthly, monthly=%d annual=%d", monthlyTotal, annualTotal)
	}
}
