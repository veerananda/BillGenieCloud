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

// CalculateOrderTax derives sub_total (excl. GST), tax_amount, and total from menu line gross.
// Exclusive: menu prices are before GST — tax is added on top; discount reduces final total after tax.
// Inclusive: menu prices include GST — bill shows taxable value + GST split; discount applies to inclusive gross.
func CalculateOrderTax(grossAmount float64, discount float64, pricesIncludeGST bool) (subTotal float64, taxAmount float64, total float64) {
	if grossAmount < 0 {
		grossAmount = 0
	}
	if discount < 0 {
		discount = 0
	}

	if pricesIncludeGST {
		net := grossAmount - discount
		if net < 0 {
			net = 0
		}
		subTotal = net / (1 + OrderGSTRate)
		taxAmount = net - subTotal
		total = net
		return subTotal, taxAmount, total
	}

	subTotal = grossAmount
	taxAmount = grossAmount * OrderGSTRate
	total = grossAmount + taxAmount - discount
	if total < 0 {
		total = 0
	}
	return subTotal, taxAmount, total
}
