package services

import "restaurant-api/internal/models"

// OrderGSTRate is the GST rate applied to restaurant orders (5%).
const OrderGSTRate = 0.05

// InitialOrderItemStatus returns the kitchen status for a new order line.
// Readily available items (water, packaged goods) skip the kitchen queue.
func InitialOrderItemStatus(menuItem models.MenuItem) string {
	if menuItem.ReadilyAvailable {
		return "served"
	}
	return "pending"
}

// RestaurantTaxSettings captures restaurant billing settings for tax calculation.
type RestaurantTaxSettings struct {
	CompositeScheme  bool
	PricesIncludeGST bool
}

// SettingsFromRestaurant builds tax settings from a restaurant record.
func SettingsFromRestaurant(r *models.Restaurant) RestaurantTaxSettings {
	if r == nil {
		return RestaurantTaxSettings{}
	}
	return RestaurantTaxSettings{
		CompositeScheme:  r.CompositeScheme,
		PricesIncludeGST: r.PricesIncludeGST,
	}
}

// orderItemsGrossSplit splits active order line totals into taxable and non-taxable gross.
func orderItemsGrossSplit(items []models.OrderItem) (taxableGross, nonTaxableGross float64) {
	for _, item := range items {
		if item.Status == "cancelled" {
			continue
		}
		isTaxable := true
		if item.MenuItem != nil {
			isTaxable = item.MenuItem.IsTaxable
		}
		if isTaxable {
			taxableGross += item.Total
		} else {
			nonTaxableGross += item.Total
		}
	}
	return taxableGross, nonTaxableGross
}

// CalculateRestaurantOrderTax derives sub_total, tax_amount, and total.
// Non-taxable item gross is never charged GST. Composite scheme restaurants have zero tax.
func CalculateRestaurantOrderTax(taxableGross, nonTaxableGross, discount float64, settings RestaurantTaxSettings) (subTotal float64, taxAmount float64, total float64) {
	if taxableGross < 0 {
		taxableGross = 0
	}
	if nonTaxableGross < 0 {
		nonTaxableGross = 0
	}
	if discount < 0 {
		discount = 0
	}

	fullGross := taxableGross + nonTaxableGross
	if discount > fullGross {
		discount = fullGross
	}

	if settings.CompositeScheme {
		total = fullGross - discount
		if total < 0 {
			total = 0
		}
		return total, 0, total
	}

	if settings.PricesIncludeGST {
		ratio := 0.0
		if fullGross > 0 {
			ratio = taxableGross / fullGross
		}
		discountedTaxable := taxableGross - discount*ratio
		discountedNonTaxable := nonTaxableGross - discount*(1-ratio)
		if discountedTaxable < 0 {
			discountedTaxable = 0
		}
		if discountedNonTaxable < 0 {
			discountedNonTaxable = 0
		}
		taxableValue := discountedTaxable / (1 + OrderGSTRate)
		taxAmount = discountedTaxable - taxableValue
		subTotal = taxableValue + discountedNonTaxable
		total = fullGross - discount
		if total < 0 {
			total = 0
		}
		return subTotal, taxAmount, total
	}

	subTotal = taxableGross + nonTaxableGross
	taxAmount = taxableGross * OrderGSTRate
	total = subTotal + taxAmount - discount
	if total < 0 {
		total = 0
	}
	return subTotal, taxAmount, total
}

// CalculateOrderTax derives sub_total (excl. GST), tax_amount, and total from menu line gross.
// Exclusive: menu prices are before GST — tax is added on top; discount reduces final total after tax.
// Inclusive: menu prices include GST — bill shows taxable value + GST split; discount applies to inclusive gross.
func CalculateOrderTax(grossAmount float64, discount float64, pricesIncludeGST bool) (subTotal float64, taxAmount float64, total float64) {
	return CalculateRestaurantOrderTax(grossAmount, 0, discount, RestaurantTaxSettings{
		PricesIncludeGST: pricesIncludeGST,
	})
}
