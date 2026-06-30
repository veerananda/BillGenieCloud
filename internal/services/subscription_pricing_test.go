package services

import "testing"

func TestCalculateSubscriptionQuoteBasic(t *testing.T) {
	quote := CalculateSubscriptionQuote(DefaultSubscriptionSelection())
	if quote.MonthlySubtotal != BasicMonthlyPriceINR {
		t.Fatalf("expected %d, got %d", BasicMonthlyPriceINR, quote.MonthlySubtotal)
	}
	if quote.BundledStaff != IncludedStaffINR {
		t.Fatalf("expected %d bundled staff, got %d", IncludedStaffINR, quote.BundledStaff)
	}
}

func TestCalculateSubscriptionQuoteTableBundle(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	sel.MaxTables = 15

	quote := CalculateSubscriptionQuote(sel)
	want := BasicMonthlyPriceINR + TableStaffBundlePriceINR
	if quote.MonthlySubtotal != want {
		t.Fatalf("expected %d, got %d", want, quote.MonthlySubtotal)
	}
	if quote.BundledStaff != 3 {
		t.Fatalf("expected 3 bundled staff at 15 tables, got %d", quote.BundledStaff)
	}
	if quote.BundledManagers != 1 {
		t.Fatalf("expected 1 bundled manager at 15 tables, got %d", quote.BundledManagers)
	}
}

func TestCalculateSubscriptionQuoteWithAddons(t *testing.T) {
	sel := DefaultSubscriptionSelection()
	sel.MaxTables = 30
	sel.ExtraStaff = 1
	sel.ExtraManagers = 1
	sel.KitchenDineIn = true
	sel.HistoryExtended = true

	quote := CalculateSubscriptionQuote(sel)
	want := BasicMonthlyPriceINR +
		4*TableStaffBundlePriceINR +
		PriceExtraStaffINR +
		PriceExtraManagerINR +
		PriceKitchenDineInINR +
		PriceHistoryExtendedINR
	if quote.MonthlySubtotal != want {
		t.Fatalf("expected %d, got %d", want, quote.MonthlySubtotal)
	}
	if quote.BundledStaff != 7 { // 2 + 4 bundles + 1 extra
		t.Fatalf("expected 7 total staff seats, got %d", quote.BundledStaff)
	}
	if quote.BundledManagers != 3 { // floor(30/15)=2 + 1 extra
		t.Fatalf("expected 3 manager seats, got %d", quote.BundledManagers)
	}
}

func TestBundledLimitsFromTables(t *testing.T) {
	if BundledStaffFromTables(20) != 4 {
		t.Fatalf("expected 4 staff at 20 tables")
	}
	if BundledManagersFromTables(20) != 1 {
		t.Fatalf("expected 1 manager at 20 tables")
	}
	if NormalizeMaxTables(17) != 15 {
		t.Fatalf("expected 17 tables to normalize to 15, got %d", NormalizeMaxTables(17))
	}
}

func TestValidateSubscriptionSelection(t *testing.T) {
	_, err := ValidateSubscriptionSelection(SubscriptionSelection{OperationMode: "invalid"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
