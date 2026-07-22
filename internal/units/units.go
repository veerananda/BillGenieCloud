package units

import (
	"fmt"
	"strings"
)

// Family groups convertible units.
type Family string

const (
	FamilyWeight Family = "weight"
	FamilyVolume Family = "volume"
	FamilyOther  Family = "other"
)

const (
	UnitGrams  = "grams"
	UnitKG     = "kg"
	UnitML     = "ml"
	UnitLiters = "liters"
	UnitPieces = "pieces"
)

// Factors relative to the family's small unit (grams / ml).
var toSmall = map[string]float64{
	UnitGrams:  1,
	UnitKG:     1000,
	UnitML:     1,
	UnitLiters: 1000,
}

// NormalizeUnit maps aliases to the canonical spelling used in the app.
func NormalizeUnit(unit string) string {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch u {
	case "g", "gram", "grams", "gm", "gms":
		return UnitGrams
	case "kg", "kgs", "kilogram", "kilograms":
		return UnitKG
	case "ml", "milliliter", "milliliters", "millilitre", "millilitres":
		return UnitML
	case "l", "lt", "ltr", "liter", "liters", "litre", "litres":
		return UnitLiters
	case "pc", "pcs", "piece", "pieces":
		return UnitPieces
	default:
		return strings.TrimSpace(unit)
	}
}

// FamilyOf returns the conversion family for a unit.
func FamilyOf(unit string) Family {
	switch NormalizeUnit(unit) {
	case UnitGrams, UnitKG:
		return FamilyWeight
	case UnitML, UnitLiters:
		return FamilyVolume
	default:
		return FamilyOther
	}
}

// SameFamily reports whether two units can convert into each other.
func SameFamily(a, b string) bool {
	fa, fb := FamilyOf(a), FamilyOf(b)
	if fa == FamilyOther || fb == FamilyOther {
		return NormalizeUnit(a) == NormalizeUnit(b) && NormalizeUnit(a) != ""
	}
	return fa == fb
}

// CanonicalUnit is the inventory storage unit for a family.
// Weight → kg, volume → liters, otherwise the normalized unit as given.
func CanonicalUnit(unit string) string {
	n := NormalizeUnit(unit)
	switch FamilyOf(n) {
	case FamilyWeight:
		return UnitKG
	case FamilyVolume:
		return UnitLiters
	default:
		if n == "" {
			return unit
		}
		return n
	}
}

// FamilyMemberUnits returns known units that belong to a family (for DB lookups).
func FamilyMemberUnits(family Family) []string {
	switch family {
	case FamilyWeight:
		return []string{UnitGrams, UnitKG, "g", "gram", "gm", "gms", "kgs", "kilogram", "kilograms"}
	case FamilyVolume:
		return []string{UnitML, UnitLiters, "l", "lt", "ltr", "liter", "litre", "litres", "milliliter", "milliliters", "millilitre", "millilitres"}
	default:
		return nil
	}
}

// Convert converts qty from one unit to another within the same family.
func Convert(qty float64, fromUnit, toUnit string) (float64, error) {
	from := NormalizeUnit(fromUnit)
	to := NormalizeUnit(toUnit)
	if from == "" || to == "" {
		return 0, fmt.Errorf("unit is required")
	}
	if from == to {
		return qty, nil
	}
	if !SameFamily(from, to) {
		return 0, fmt.Errorf("cannot convert %s to %s", fromUnit, toUnit)
	}
	if FamilyOf(from) == FamilyOther {
		return qty, nil
	}
	fromFactor, okFrom := toSmall[from]
	toFactor, okTo := toSmall[to]
	if !okFrom || !okTo || fromFactor <= 0 || toFactor <= 0 {
		return 0, fmt.Errorf("unsupported unit conversion %s → %s", fromUnit, toUnit)
	}
	return qty * fromFactor / toFactor, nil
}

// ToCanonical converts a quantity into the inventory storage unit for that family.
func ToCanonical(qty float64, unit string) (float64, string, error) {
	canonical := CanonicalUnit(unit)
	converted, err := Convert(qty, unit, canonical)
	if err != nil {
		return 0, "", err
	}
	return converted, canonical, nil
}

// EntryUnitsFor returns the restock/entry unit choices for an inventory unit.
func EntryUnitsFor(inventoryUnit string) []string {
	switch FamilyOf(inventoryUnit) {
	case FamilyWeight:
		return []string{UnitGrams, UnitKG}
	case FamilyVolume:
		return []string{UnitML, UnitLiters}
	default:
		u := NormalizeUnit(inventoryUnit)
		if u == "" {
			return nil
		}
		return []string{u}
	}
}
