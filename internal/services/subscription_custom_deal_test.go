package services

import "testing"

func TestValidateCustomDealRequestDefaults(t *testing.T) {
	req, err := ValidateCustomDealRequest(CustomDealRequest{Notes: " need 40 tables "})
	if err != nil {
		t.Fatal(err)
	}
	if req.MaxTables != 0 {
		t.Fatalf("expected no forced table capacity on empty request, got %d", req.MaxTables)
	}
	if req.BillingCycle != BillingCycleQuarterly {
		t.Fatalf("expected quarterly cycle, got %s", req.BillingCycle)
	}
	if req.Notes != "need 40 tables" {
		t.Fatalf("notes not trimmed: %q", req.Notes)
	}
	if req.Status != CustomDealRequestPending {
		t.Fatalf("expected pending status")
	}
}

func TestHasPendingCustomDealRequest(t *testing.T) {
	cfg := StoredSubscriptionConfig{
		CustomDealRequest: &CustomDealRequest{Status: CustomDealRequestPending, MaxTables: 40},
	}
	if !HasPendingCustomDealRequest(cfg) {
		t.Fatal("expected pending")
	}
	cfg.CustomDealRequest.Status = CustomDealRequestFulfilled
	if HasPendingCustomDealRequest(cfg) {
		t.Fatal("fulfilled should not be pending")
	}
}

func TestQuoteFromCustomDeal(t *testing.T) {
	deal := CustomDeal{
		MonthlyPrice: 4999,
		Selection: SubscriptionSelection{
			BillingCycle:  BillingCycleQuarterly,
			OperationMode: "both",
			MaxTables:     40,
			ExtraStaff:    10,
			ExtraChefs:    2,
			ExtraManagers: 1,
			Inventory:     true,
			Expenses:      true,
		},
	}
	q := QuoteFromCustomDeal(deal)
	if q.MonthlySubtotal != 4999 {
		t.Fatalf("monthly=%d", q.MonthlySubtotal)
	}
	if q.AnnualTotal != 4999*AnnualMultiplier {
		t.Fatalf("annual=%d", q.AnnualTotal)
	}
	if len(q.LineItems) != 1 || q.LineItems[0].ID != "custom_deal" {
		t.Fatalf("expected custom_deal line item")
	}
}

func TestLimitsFromConfigCustomDeal(t *testing.T) {
	tables := 60
	deal := CustomDeal{
		MonthlyPrice: 7999,
		Selection: SubscriptionSelection{
			BillingCycle:    BillingCycleQuarterly,
			MaxTables:       40,
			ExtraStaff:      5,
			ExtraChefs:      3,
			ExtraManagers:   2,
			Inventory:       true,
			Expenses:        true,
			HistoryExtended: true,
			KitchenDineIn:   true,
			KitchenCounter:  true,
			OperationMode:   "both",
		},
		LimitsOverride:       &CustomLimitsOverride{MaxTables: &tables},
		LockSelfServeChanges: true,
	}
	cfg := StoredSubscriptionConfig{
		PricingMode: PricingModeCustom,
		CustomDeal:  &deal,
		Selection:   deal.Selection,
	}
	limits := LimitsFromConfig(cfg, 0)
	if limits.MaxTables != 60 {
		t.Fatalf("expected override tables 60, got %d", limits.MaxTables)
	}
	if limits.MaxStaff != IncludedStaffINR+5 {
		t.Fatalf("staff=%d", limits.MaxStaff)
	}
	if limits.HistoryDays != ExtendedHistoryDays {
		t.Fatalf("history=%d", limits.HistoryDays)
	}
	if !limits.Inventory || !limits.Expenses {
		t.Fatalf("expected inventory+expenses")
	}
	if limits.MonthlyPrice != 7999 {
		t.Fatalf("price=%d", limits.MonthlyPrice)
	}
	if !SelfServePlanChangesLocked(cfg) {
		t.Fatal("expected lock")
	}
}

func TestValidateCustomDealRejectsZeroPrice(t *testing.T) {
	_, err := ValidateCustomDeal(CustomDeal{MonthlyPrice: 0})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCanChangePlanMidCycleLockedByCustomDeal(t *testing.T) {
	cfg := StoredSubscriptionConfig{
		Phase:       SubscriptionPhaseActive,
		PricingMode: PricingModeCustom,
		CustomDeal: &CustomDeal{
			MonthlyPrice:         3000,
			LockSelfServeChanges: true,
			Selection:            DefaultSubscriptionSelection(),
		},
	}
	if !SelfServePlanChangesLocked(cfg) {
		t.Fatal("expected locked")
	}
}
