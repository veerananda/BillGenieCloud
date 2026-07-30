package services

import "testing"

func TestQuoteFromCustomDeal(t *testing.T) {
	deal := CustomDeal{
		MonthlyPrice: 4999,
		Selection: SubscriptionSelection{
			BillingCycle:  "monthly",
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
			BillingCycle:     "monthly",
			MaxTables:        40,
			ExtraStaff:       5,
			ExtraChefs:       3,
			ExtraManagers:    2,
			Inventory:        true,
			Expenses:         true,
			HistoryExtended:  true,
			KitchenDineIn:    true,
			KitchenCounter:   true,
			OperationMode:    "both",
		},
		LimitsOverride: &CustomLimitsOverride{MaxTables: &tables},
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
