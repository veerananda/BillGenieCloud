package services

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildBillPDF_LooksLikeReceipt(t *testing.T) {
	pdf, err := BuildBillPDF(BillSummaryView{
		RestaurantName: "Test Kitchen",
		Address:        "12 MG Road",
		ContactNumber:  "9876543210",
		GstNumber:      "29AAAAA0000A1Z5",
		TableNumber:    "5",
		OrderNumber:    42,
		TicketNumber:   42,
		Items: []BillItemView{
			{Name: "Masala Dosa", Quantity: 2, UnitRate: 80, Total: 160},
			{Name: "Filter Coffee", Quantity: 1, UnitRate: 40, Total: 40},
		},
		SubTotal:  190.48,
		TaxAmount: 9.52,
		Total:     200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("missing PDF header")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatalf("missing PDF trailer")
	}
	body := string(pdf)
	for _, want := range []string{"Test Kitchen", "Masala Dosa", "Total", "Rs."} {
		if !strings.Contains(body, want) {
			t.Fatalf("PDF missing %q", want)
		}
	}
}
