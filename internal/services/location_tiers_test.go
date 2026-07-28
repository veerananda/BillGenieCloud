package services

import "testing"

func TestResolveCityTier(t *testing.T) {
	tier, ok := ResolveCityTier("Karnataka", "Bengaluru")
	if !ok {
		t.Fatal("expected Karnataka/Bengaluru to resolve")
	}
	if tier != CityTier1 {
		t.Fatalf("expected tier_1, got %s", tier)
	}

	tier, ok = ResolveCityTier("Maharashtra", "Pune")
	if !ok || tier != CityTier1 {
		t.Fatalf("expected Maharashtra/Pune tier_1, got %s ok=%v", tier, ok)
	}

	tier, ok = ResolveCityTier("Gujarat", "Surat")
	if !ok || tier != CityTier2 {
		t.Fatalf("expected Gujarat/Surat tier_2, got %s ok=%v", tier, ok)
	}

	tier, ok = ResolveCityTier("Karnataka", "Other city")
	if !ok {
		t.Fatal("expected fallback city to resolve")
	}
	if tier != CityTier3 {
		t.Fatalf("expected tier_3 fallback, got %s", tier)
	}

	if _, ok := ResolveCityTier("Karnataka", "Mumbai"); ok {
		t.Fatal("expected invalid city/state combination to fail")
	}
}

func TestIndiaLocationOptionsCoverOfficialXCities(t *testing.T) {
	want := map[string]string{
		"Ahmedabad":  "Gujarat",
		"Bengaluru":  "Karnataka",
		"Chennai":    "Tamil Nadu",
		"Delhi":      "Delhi",
		"Hyderabad":  "Telangana",
		"Kolkata":    "West Bengal",
		"Mumbai":     "Maharashtra",
		"Pune":       "Maharashtra",
	}
	for city, state := range want {
		tier, ok := ResolveCityTier(state, city)
		if !ok || tier != CityTier1 {
			t.Fatalf("%s/%s should be tier_1, got %s ok=%v", state, city, tier, ok)
		}
	}
}
