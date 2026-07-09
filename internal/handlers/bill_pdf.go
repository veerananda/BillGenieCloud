package handlers

import (
	"bytes"
	"fmt"
	"strings"

	"restaurant-api/internal/services"

	"github.com/go-pdf/fpdf"
)

func buildBillPDF(summary services.BillSummaryView) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	// Header band
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetXY(18, 10)
	pdf.CellFormat(0, 8, "BILL SUMMARY", "", 1, "L", false, 0, "")

	pdf.SetTextColor(15, 23, 42)
	pdf.Ln(10)

	title := summary.RestaurantName
	if title == "" {
		title = "BillGenie"
	}
	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(0, 10, title, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(100, 116, 139)
	meta := fmt.Sprintf("Order #%d", summary.OrderNumber)
	if summary.TableNumber != "" {
		meta += fmt.Sprintf("  |  Table %s", summary.TableNumber)
	}
	pdf.CellFormat(0, 6, meta, "", 1, "C", false, 0, "")

	if dateLine := formatBillDateTime(summary.CreatedAt); dateLine != "" {
		pdf.CellFormat(0, 6, dateLine, "", 1, "C", false, 0, "")
	}
	if summary.CustomerName != "" && summary.CustomerName != "Guest" &&
		summary.CustomerName != "Takeaway" && summary.CustomerName != "Counter" &&
		summary.CustomerName != "Self Service" {
		pdf.CellFormat(0, 6, "Customer: "+summary.CustomerName, "", 1, "C", false, 0, "")
	}

	pdf.Ln(6)
	pdf.SetDrawColor(226, 232, 240)
	pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
	pdf.Ln(8)

	// Items table header
	pdf.SetFillColor(248, 250, 252)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.CellFormat(95, 8, "ITEM", "B", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "QTY", "B", 0, "C", true, 0, "")
	pdf.CellFormat(57, 8, "AMOUNT", "B", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(15, 23, 42)
	for _, item := range summary.Items {
		name := item.Name
		if len(name) > 42 {
			name = name[:39] + "..."
		}
		pdf.CellFormat(95, 8, name, "B", 0, "L", false, 0, "")
		pdf.CellFormat(20, 8, fmt.Sprintf("%d", item.Quantity), "B", 0, "C", false, 0, "")
		pdf.CellFormat(57, 8, formatBillCurrency(item.Total), "B", 1, "R", false, 0, "")
	}

	pdf.Ln(4)
	pdf.SetDrawColor(226, 232, 240)
	pdf.Line(120, pdf.GetY(), 192, pdf.GetY())
	pdf.Ln(6)

	writeTotalRow := func(label, value string, bold bool) {
		if bold {
			pdf.SetFont("Helvetica", "B", 12)
			pdf.SetTextColor(15, 23, 42)
		} else {
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(71, 85, 105)
		}
		pdf.CellFormat(102, 7, label, "", 0, "R", false, 0, "")
		pdf.CellFormat(70, 7, value, "", 1, "R", false, 0, "")
	}

	if summary.SubTotal > 0 {
		writeTotalRow(subtotalLabelBill(summary.PricesIncludeGST), formatBillCurrency(summary.SubTotal), false)
	}
	if summary.TaxAmount > 0 {
		writeTotalRow("GST (5%)", formatBillCurrency(summary.TaxAmount), false)
	}
	if summary.DiscountAmount > 0 {
		pdf.SetTextColor(22, 163, 74)
		writeTotalRow("Discount", "-"+formatBillCurrency(summary.DiscountAmount), false)
	}
	pdf.Ln(2)
	writeTotalRow("Total", formatBillCurrency(summary.Total), true)

	if summary.IsPaid && summary.PaymentMethod != "" {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(71, 85, 105)
		writeTotalRow("Payment", strings.ToUpper(summary.PaymentMethod), false)
	}

	pdf.Ln(12)
	pdf.SetFont("Helvetica", "I", 10)
	pdf.SetTextColor(148, 163, 184)
	pdf.CellFormat(0, 6, "Thank you for dining with us.", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
