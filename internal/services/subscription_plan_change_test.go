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

func TestPeriodAmountUsesCycles(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	sel.BillingCycle = BillingCycleAnnual
	sel.Inventory = true
	q := CalculateSubscriptionQuote(sel)
	_, _, quarterlyTotal := periodAmountINR(q, BillingCycleQuarterly)
	_, _, halfYearlyTotal := periodAmountINR(q, BillingCycleHalfYearly)
	_, _, annualTotal := periodAmountINR(q, BillingCycleAnnual)
	if halfYearlyTotal <= quarterlyTotal {
		t.Fatalf("half-yearly should exceed quarterly, q=%d h=%d", quarterlyTotal, halfYearlyTotal)
	}
	if annualTotal <= halfYearlyTotal {
		t.Fatalf("annual should exceed half-yearly, h=%d a=%d", halfYearlyTotal, annualTotal)
	}
}
