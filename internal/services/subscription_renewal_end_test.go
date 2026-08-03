package services

import (
	"testing"
	"time"
)

func TestNextSubscriptionEndPreservesUnusedDays(t *testing.T) {
	now := time.Now()
	futureEnd := now.AddDate(0, 0, 5)

	got := NextSubscriptionEnd(futureEnd, BillingCycleQuarterly)
	want := futureEnd.AddDate(0, 3, 0)
	if !got.Equal(want) {
		t.Fatalf("expected unused days preserved: want %v, got %v", want, got)
	}
}

func TestNextSubscriptionEndFromNowWhenExpired(t *testing.T) {
	now := time.Now()
	pastEnd := now.AddDate(0, 0, -3)

	got := NextSubscriptionEnd(pastEnd, BillingCycleQuarterly)
	wantApprox := now.AddDate(0, 3, 0)
	if got.Before(wantApprox.Add(-2*time.Minute)) || got.After(wantApprox.Add(2*time.Minute)) {
		t.Fatalf("expected ~1 quarter from now when expired, got %v (now=%v)", got, now)
	}
}

func TestNextSubscriptionEndAnnual(t *testing.T) {
	now := time.Now()
	got := NextSubscriptionEnd(time.Time{}, BillingCycleAnnual)
	wantApprox := now.AddDate(1, 0, 0)
	if got.Before(wantApprox.Add(-2*time.Minute)) || got.After(wantApprox.Add(2*time.Minute)) {
		t.Fatalf("expected ~1 year from now for annual, got %v", got)
	}
}

func TestNextSubscriptionEndHalfYearly(t *testing.T) {
	now := time.Now()
	got := NextSubscriptionEnd(time.Time{}, BillingCycleHalfYearly)
	wantApprox := now.AddDate(0, 6, 0)
	if got.Before(wantApprox.Add(-2*time.Minute)) || got.After(wantApprox.Add(2*time.Minute)) {
		t.Fatalf("expected ~6 months from now for half-yearly, got %v", got)
	}
}

func TestNormalizeBillingCycleLegacyMonthly(t *testing.T) {
	if NormalizeBillingCycle("monthly") != BillingCycleQuarterly {
		t.Fatal("legacy monthly should map to quarterly")
	}
}
