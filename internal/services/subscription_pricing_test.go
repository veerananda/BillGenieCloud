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
	if quote.BundledStaff != IncludedStaffINR {
		t.Fatalf("expected %d bundled staff, got %d", IncludedStaffINR, quote.BundledStaff)
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

	scale := DefaultSubscriptionSelection()
	scale.MaxTables = 25
	sq := CalculateSubscriptionQuote(scale)
	if sq.PlanBand != PlanBandScale || sq.MonthlySubtotal != ScaleMonthlyTier2INR {
		t.Fatalf("scale: band=%s price=%d", sq.PlanBand, sq.MonthlySubtotal)
	}
}

func TestCalculateSubscriptionQuoteWithAddons(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	sel.ExtraStaff = 2
	sel.ExtraChefs = 1
	sel.ExtraManagers = 1
	sel.HistoryExtended = true
	sel.Inventory = true
	sel.Expenses = true

	quote := CalculateSubscriptionQuote(sel)
	want := StarterMonthlyTier2INR +
		2*PriceExtraStaffINR +
		PriceExtraChefINR +
		PriceExtraManagerINR +
		PriceHistoryExtendedINR +
		PriceInventoryINR +
		PriceExpensesINR
	if quote.MonthlySubtotal != want {
		t.Fatalf("expected %d, got %d", want, quote.MonthlySubtotal)
	}
	if quote.BundledStaff != IncludedStaffINR+2 {
		t.Fatalf("expected %d staff seats, got %d", IncludedStaffINR+2, quote.BundledStaff)
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

func TestExtraSeatCaps(t *testing.T) {
	sel, err := ValidateSubscriptionSelection(SubscriptionSelection{
		MaxTables:     10,
		ExtraStaff:    99,
		ExtraChefs:    99,
		ExtraManagers: 99,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sel.ExtraStaff != MaxExtraStaff || sel.ExtraChefs != MaxExtraChefs || sel.ExtraManagers != MaxExtraManagers {
		t.Fatalf("caps: staff=%d chefs=%d managers=%d", sel.ExtraStaff, sel.ExtraChefs, sel.ExtraManagers)
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
	if limits.MaxStaff != IncludedStaffINR+1 {
		t.Fatalf("max staff: got %d", limits.MaxStaff)
	}
	if limits.MaxChefs != IncludedChefsINR+2 {
		t.Fatalf("max chefs: got %d", limits.MaxChefs)
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
