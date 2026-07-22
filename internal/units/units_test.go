package units

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestNormalizeAndCanonical(t *testing.T) {
	cases := []struct {
		in, norm, canon string
	}{
		{"g", UnitGrams, UnitKG},
		{"grams", UnitGrams, UnitKG},
		{"KG", UnitKG, UnitKG},
		{"ml", UnitML, UnitLiters},
		{"L", UnitLiters, UnitLiters},
		{"pieces", UnitPieces, UnitPieces},
		{"pinch", "pinch", "pinch"},
	}
	for _, tc := range cases {
		if got := NormalizeUnit(tc.in); got != tc.norm {
			t.Fatalf("NormalizeUnit(%q)=%q want %q", tc.in, got, tc.norm)
		}
		if got := CanonicalUnit(tc.in); got != tc.canon {
			t.Fatalf("CanonicalUnit(%q)=%q want %q", tc.in, got, tc.canon)
		}
	}
}

func TestConvert(t *testing.T) {
	got, err := Convert(500, UnitGrams, UnitKG)
	if err != nil || !almostEqual(got, 0.5) {
		t.Fatalf("500g→kg got %v err %v", got, err)
	}
	got, err = Convert(1, UnitKG, UnitGrams)
	if err != nil || !almostEqual(got, 1000) {
		t.Fatalf("1kg→g got %v err %v", got, err)
	}
	got, err = Convert(250, UnitML, UnitLiters)
	if err != nil || !almostEqual(got, 0.25) {
		t.Fatalf("250ml→L got %v err %v", got, err)
	}
	if _, err := Convert(1, UnitGrams, UnitML); err == nil {
		t.Fatal("expected cross-family error")
	}
}

func TestToCanonical(t *testing.T) {
	qty, unit, err := ToCanonical(50, "grams")
	if err != nil || unit != UnitKG || !almostEqual(qty, 0.05) {
		t.Fatalf("got %v %q err %v", qty, unit, err)
	}
}
