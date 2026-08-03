package services

import "testing"

func TestCalculateSubscriptionQuoteStarter(t *testing.T) {
	quote := CalculateSubscriptionQuote(DefaultSubscriptionSelection())
	if quote.MonthlySubtotal != StarterMonthlyTier2INR {
		t.Fatalf("expected %d, got %d", StarterMonthlyTier2INR, quote.MonthlySubtotal)
	}
	if quote.PlanBand != PlanBandStarter {
		t.Fatalf("expected starter band, got %s", quote.PlanBand)
	}
	if quote.BundledStaff != StarterStaffINR || quote.BundledChefs != StarterChefsINR {
		t.Fatalf("starter seats: staff=%d chefs=%d", quote.BundledStaff, quote.BundledChefs)
	}
	if quote.Selection.MaxTables != PlanStarterTables {
		t.Fatalf("expected %d tables, got %d", PlanStarterTables, quote.Selection.MaxTables)
	}
}

func TestCalculateSubscriptionQuoteGrowthAndScale(t *testing.T) {
	growth := DefaultSubscriptionSelection()
	growth.MaxTables = 15
	gq := CalculateSubscriptionQuote(growth)
	if gq.PlanBand != PlanBandGrowth || gq.Selection.MaxTables != PlanGrowthTables {
		t.Fatalf("expected growth/%d, got %s/%d", PlanGrowthTables, gq.PlanBand, gq.Selection.MaxTables)
	}
	if gq.MonthlySubtotal != GrowthMonthlyTier2INR {
		t.Fatalf("growth price: got %d want %d", gq.MonthlySubtotal, GrowthMonthlyTier2INR)
	}
	if gq.BundledStaff != GrowthStaffINR || gq.BundledChefs != GrowthChefsINR {
		t.Fatalf("growth seats: staff=%d chefs=%d", gq.BundledStaff, gq.BundledChefs)
	}

	scale := DefaultSubscriptionSelection()
	scale.MaxTables = 25
	sq := CalculateSubscriptionQuote(scale)
	if sq.PlanBand != PlanBandScale || sq.MonthlySubtotal != ScaleMonthlyTier2INR {
		t.Fatalf("scale: band=%s price=%d", sq.PlanBand, sq.MonthlySubtotal)
	}
	if sq.BundledStaff != ScaleStaffINR || sq.BundledChefs != ScaleChefsINR {
		t.Fatalf("scale seats: staff=%d chefs=%d", sq.BundledStaff, sq.BundledChefs)
	}
}

func TestCalculateSubscriptionQuoteWithAddons(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	// Catalog ignores seat extras — only optional add-ons are charged.
	sel.ExtraStaff = 2
	sel.ExtraChefs = 1
	sel.ExtraManagers = 1
	sel.HistoryExtended = true
	sel.Inventory = true
	sel.Expenses = true

	quote := CalculateSubscriptionQuote(sel)
	want := StarterMonthlyTier2INR +
		PriceHistoryExtendedINR +
		PriceInventoryINR +
		PriceExpensesINR
	if quote.MonthlySubtotal != want {
		t.Fatalf("expected %d, got %d", want, quote.MonthlySubtotal)
	}
	if quote.BundledStaff != StarterStaffINR {
		t.Fatalf("expected %d staff seats, got %d", StarterStaffINR, quote.BundledStaff)
	}
	if quote.Selection.ExtraStaff != 0 {
		t.Fatalf("catalog extras should be cleared, got %d", quote.Selection.ExtraStaff)
	}
}

func TestNormalizeMaxTablesBands(t *testing.T) {
	if NormalizeMaxTables(0) != PlanStarterTables {
		t.Fatalf("default: got %d", NormalizeMaxTables(0))
	}
	if NormalizeMaxTables(8) != PlanStarterTables {
		t.Fatalf("8 -> starter ceiling")
	}
	if NormalizeMaxTables(12) != PlanGrowthTables {
		t.Fatalf("12 -> growth")
	}
	if NormalizeMaxTables(20) != PlanScaleTables {
		t.Fatalf("20 -> scale")
	}
	if NormalizeMaxTables(50) != PlanScaleTables {
		t.Fatalf("50 catalog-capped at scale")
	}
}

func TestValidateSubscriptionSelectionNormalizesBase(t *testing.T) {
	sel, err := ValidateSubscriptionSelection(SubscriptionSelection{
		BillingCycle:  "monthly",
		OperationMode: "dine_in",
		MaxTables:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.BillingCycle != BillingCycleQuarterly {
		t.Fatalf("legacy monthly should normalize to quarterly, got %s", sel.BillingCycle)
	}
	if sel.OperationMode != "both" || sel.MaxTables != PlanStarterTables {
		t.Fatalf("expected both + %d tables, got %s / %d", PlanStarterTables, sel.OperationMode, sel.MaxTables)
	}
	if !sel.KitchenDineIn || !sel.KitchenCounter {
		t.Fatal("expected kitchen included")
	}
}

func TestValidateSubscriptionSelectionRejectsBadCycle(t *testing.T) {
	_, err := ValidateSubscriptionSelection(SubscriptionSelection{BillingCycle: "weekly"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCatalogClearsSeatExtras(t *testing.T) {
	sel, err := ValidateSubscriptionSelection(SubscriptionSelection{
		MaxTables:     10,
		ExtraStaff:    99,
		ExtraChefs:    99,
		ExtraManagers: 99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.ExtraStaff != 0 || sel.ExtraChefs != 0 || sel.ExtraManagers != 0 {
		t.Fatalf("extras should be 0, got staff=%d chefs=%d managers=%d", sel.ExtraStaff, sel.ExtraChefs, sel.ExtraManagers)
	}
}

func TestCalculateSubscriptionQuoteCityTiers(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	tier1 := CalculateSubscriptionQuoteForTier(sel, CityTier1)
	tier2 := CalculateSubscriptionQuoteForTier(sel, CityTier2)
	tier3 := CalculateSubscriptionQuoteForTier(sel, CityTier3)

	if tier1.MonthlySubtotal != StarterMonthlyTier1INR {
		t.Fatalf("expected tier 1 monthly %d, got %d", StarterMonthlyTier1INR, tier1.MonthlySubtotal)
	}
	if tier2.MonthlySubtotal != StarterMonthlyTier2INR {
		t.Fatalf("expected tier 2 monthly %d, got %d", StarterMonthlyTier2INR, tier2.MonthlySubtotal)
	}
	if tier3.MonthlySubtotal != StarterMonthlyTier3INR {
		t.Fatalf("expected tier 3 monthly %d, got %d", StarterMonthlyTier3INR, tier3.MonthlySubtotal)
	}
	if tier2.AnnualTotal != tier2.MonthlySubtotal*AnnualMultiplier {
		t.Fatalf("expected annual total to use %d-month pricing", AnnualMultiplier)
	}
}

func TestLimitsFromSelection(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	sel.MaxTables = 18
	sel.ExtraStaff = 1
	sel.ExtraChefs = 2
	sel.Expenses = true
	limits := LimitsFromSelection(sel, 0)
	if limits.MaxTables != PlanGrowthTables {
		t.Fatalf("max tables: got %d", limits.MaxTables)
	}
	if limits.MaxStaff != GrowthStaffINR {
		t.Fatalf("max staff: got %d want %d", limits.MaxStaff, GrowthStaffINR)
	}
	if limits.MaxChefs != GrowthChefsINR {
		t.Fatalf("max chefs: got %d want %d", limits.MaxChefs, GrowthChefsINR)
	}
	if limits.HistoryDays != IncludedHistoryDaysINR {
		t.Fatalf("history days: got %d", limits.HistoryDays)
	}
	if !limits.Expenses || limits.Inventory {
		t.Fatal("expenses/inventory flags wrong")
	}
	if !limits.KitchenDineIn || !limits.CounterEnabled {
		t.Fatal("kitchen/modes should be included")
	}
}

func TestSubscriptionPlanFromSelection(t *testing.T) {
	if SubscriptionPlanFromSelection(SubscriptionSelection{MaxTables: 10}) != PlanBandStarter {
		t.Fatal("starter")
	}
	if SubscriptionPlanFromSelection(SubscriptionSelection{MaxTables: 18}) != PlanBandGrowth {
		t.Fatal("growth")
	}
	if SubscriptionPlanFromSelection(SubscriptionSelection{MaxTables: 25}) != PlanBandScale {
		t.Fatal("scale")
	}
}
