package services

import (
	"encoding/json"
	"strings"
)

// Built-in menu section labels that should not be appended to dish names.
var builtinCategoryDisplayBlocklist = map[string]struct{}{
	"main course": {}, "main courses": {}, "mains": {},
	"starter": {}, "starters": {},
	"appetizer": {}, "appetizers": {}, "starter/appetizer": {},
	"beverage": {}, "beverages": {}, "drink": {}, "drinks": {},
	"dessert": {}, "desserts": {}, "sweet": {}, "sweets": {},
	"snack": {}, "snacks": {},
	"bread": {}, "breads": {}, "roti": {}, "rotis": {},
	"combo": {}, "combos": {},
	"special": {}, "specials": {},
	"other": {}, "others": {}, "miscellaneous": {}, "misc": {},
	"addon": {}, "add-on": {}, "add ons": {}, "add-ons": {},
	"side": {}, "sides": {}, "accompaniment": {}, "accompaniments": {},
}

func normalizeCategoryKey(category string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(category))), " ")
}

// ParseCategoryDisplayBlocklist parses the restaurant JSON array of extra blocked labels.
func ParseCategoryDisplayBlocklist(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil
	}
	return NormalizeCategoryDisplayBlocklist(list)
}

// EncodeCategoryDisplayBlocklist stores a clean JSON array for the restaurant column.
func EncodeCategoryDisplayBlocklist(list []string) string {
	clean := NormalizeCategoryDisplayBlocklist(list)
	b, err := json.Marshal(clean)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// NormalizeCategoryDisplayBlocklist trims, dedupes (case-insensitive), and drops empties.
func NormalizeCategoryDisplayBlocklist(list []string) []string {
	out := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, item := range list {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeCategoryKey(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// IsBlockedDisplayCategory reports whether category is a section label (built-in or custom).
func IsBlockedDisplayCategory(category string, extraBlocklist []string) bool {
	key := normalizeCategoryKey(category)
	if key == "" {
		return false
	}
	if _, ok := builtinCategoryDisplayBlocklist[key]; ok {
		return true
	}
	for _, extra := range extraBlocklist {
		if normalizeCategoryKey(extra) == key {
			return true
		}
	}
	return false
}

// FormatItemDisplayName builds a customer-facing dish label once.
// Dish-type categories (Biryani, Pulao) append; blocked section labels do not.
func FormatItemDisplayName(itemName, categoryName, variantLabel string, extraBlocklist []string) string {
	name := strings.TrimSpace(itemName)
	category := strings.TrimSpace(categoryName)

	base := name
	if name == "" {
		base = category
		if base == "" {
			base = "Unknown Item"
		}
	} else if category != "" {
		nameLower := strings.ToLower(name)
		categoryLower := strings.ToLower(category)
		switch {
		case nameLower == categoryLower:
			base = name
		case strings.Contains(nameLower, categoryLower):
			base = name
		case IsBlockedDisplayCategory(category, extraBlocklist):
			base = name
		default:
			base = name + " " + category
		}
	}

	label := strings.TrimSpace(variantLabel)
	if label == "" || strings.EqualFold(label, "regular") {
		return base
	}
	suffix := " (" + label + ")"
	if strings.Contains(base, suffix) {
		return base
	}
	return base + suffix
}
