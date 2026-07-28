package services

import "testing"

func TestFormatItemDisplayName(t *testing.T) {
	cases := []struct {
		name, category, variant string
		extra                   []string
		want                    string
	}{
		{"Veg", "Biryani", "", nil, "Veg Biryani"},
		{"chitti mutyalu chicken", "Pulao", "", nil, "chitti mutyalu chicken Pulao"},
		{"Paneer Butter Masala", "Main Course", "", nil, "Paneer Butter Masala"},
		{"Chicken Biryani", "Biryani", "", nil, "Chicken Biryani"},
		{"Veg", "Biryani", "Half", nil, "Veg Biryani (Half)"},
		{"Special Thali", "House Special", "", []string{"House Special"}, "Special Thali"},
		{"Special Thali", "House Special", "", nil, "Special Thali House Special"},
	}
	for _, tc := range cases {
		got := FormatItemDisplayName(tc.name, tc.category, tc.variant, tc.extra)
		if got != tc.want {
			t.Fatalf("%q + %q → got %q want %q", tc.name, tc.category, got, tc.want)
		}
	}
}
