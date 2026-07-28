package services

import "testing"

func TestResolveCityTier(t *testing.T) {
	tier, ok := ResolveCityTier("Karnataka", "Bengaluru Urban")
	if !ok {
		t.Fatal("expected Karnataka/Bengaluru Urban to resolve")
	}
	if tier != CityTier1 {
		t.Fatalf("expected tier_1, got %s", tier)
	}

	tier, ok = ResolveCityTier("Karnataka", "Other district")
	if !ok {
		t.Fatal("expected fallback district to resolve")
	}
	if tier != CityTier3 {
		t.Fatalf("expected tier_3 fallback, got %s", tier)
	}

	if _, ok := ResolveCityTier("Karnataka", "Mumbai City"); ok {
		t.Fatal("expected invalid district/state combination to fail")
	}
}
