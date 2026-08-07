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
	for _, want := range []string{"Test Kitchen", "Masala Dosa", "Total", "Rs.", "Powered by BillGenie", "BILL SUMMARY"} {
		if !strings.Contains(body, want) {
			t.Fatalf("PDF missing %q", want)
		}
	}
}

func TestHelveticaStringWidth_CentersTitles(t *testing.T) {
	// Bold uppercase should be wider than the old len*0.48 heuristic.
	w := helveticaStringWidth("BILL SUMMARY", 8, true)
	old := float64(len("BILL SUMMARY")) * 8 * 0.48
	if w <= old {
		t.Fatalf("expected AFM width %.1f > crude %.1f", w, old)
	}
	nameW := helveticaStringWidth("Test Kitchen", 14, true)
	if nameW <= 0 {
		t.Fatal("restaurant name width should be positive")
	}
}
