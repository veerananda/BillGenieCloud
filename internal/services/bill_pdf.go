package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// BuildBillPDF renders a customer bill PDF that mirrors the on-screen HTML receipt
// (narrow sheet, header, item table, totals). Uses core PDF fonts; currency as Rs.
func BuildBillPDF(summary BillSummaryView) ([]byte, error) {
	title := strings.TrimSpace(summary.RestaurantName)
	if title == "" {
		title = "BillGenie"
	}

	// Narrow receipt page (~HTML max-width 420px).
	const pageW = 340.0
	const pageH = 842.0
	const left = 28.0
	const right = pageW - 28.0
	const contentW = right - left

	var body strings.Builder
	y := pageH - 36

	writeText := func(text string, size, x, baseline float64, bold bool) {
		font := "F1"
		if bold {
			font = "F2"
		}
		body.WriteString(fmt.Sprintf("BT /%s %.1f Tf %.1f %.1f Td (%s) Tj ET\n",
			font, size, x, baseline, pdfEscape(text)))
	}
	approxWidth := func(text string, size float64) float64 {
		// Helvetica average glyph width ~0.5em for mixed case.
		return float64(len(text)) * size * 0.48
	}
	centerText := func(text string, size float64, bold bool) {
		w := approxWidth(text, size)
		x := left + (contentW-w)/2
		if x < left {
			x = left
		}
		writeText(text, size, x, y, bold)
		y -= size + 5
	}
	leftText := func(text string, size float64, bold bool) {
		writeText(text, size, left, y, bold)
		y -= size + 5
	}
	hline := func() {
		body.WriteString("0.75 w 0.85 0.88 0.91 RG\n")
		body.WriteString(fmt.Sprintf("%.1f %.1f m %.1f %.1f l S\n", left, y, right, y))
		body.WriteString("0 g\n")
		y -= 10
	}
	space := func(dy float64) { y -= dy }

	centerText("BILL SUMMARY", 8, true)
	space(2)
	centerText(title, 14, true)

	if a := strings.TrimSpace(summary.Address); a != "" {
		centerText(a, 9, false)
	}
	if c := strings.TrimSpace(summary.ContactNumber); c != "" {
		centerText(c, 9, false)
	}
	if g := strings.TrimSpace(summary.GstNumber); g != "" {
		centerText("GSTIN: "+g, 9, false)
	}

	meta := billPDFMetaLine(summary)
	if meta != "" {
		centerText(meta, 9, false)
	}
	if !summary.CreatedAt.IsZero() {
		centerText(summary.CreatedAt.In(time.Local).Format("02 Jan 2006, 03:04 PM"), 9, false)
	}
	if n := strings.TrimSpace(summary.CustomerName); n != "" &&
		n != "Guest" && n != "Takeaway" && n != "Counter" && n != "Self Service" {
		centerText("Customer: "+n, 9, false)
	}
	if p := strings.TrimSpace(summary.CustomerPhone); p != "" {
		centerText("Phone: "+p, 9, false)
	}
	if a := strings.TrimSpace(summary.AttendedByName); a != "" {
		centerText("Attended by: "+a, 9, false)
	}

	space(8)
	hline()

	colQty := left + contentW*0.52
	colRate := left + contentW*0.68
	colPrice := left + contentW*0.84

	writeText("ITEM", 7, left, y, true)
	writeText("QTY", 7, colQty, y, true)
	writeText("RATE", 7, colRate, y, true)
	writeText("PRICE", 7, colPrice, y, true)
	y -= 12
	hline()

	for _, item := range summary.Items {
		if y < 90 {
			break
		}
		rate := item.UnitRate
		if rate <= 0 && item.Quantity > 0 {
			rate = item.Total / float64(item.Quantity)
		}
		name := truncatePDF(item.Name, 28)
		writeText(name, 9, left, y, false)
		writeText(fmt.Sprintf("%d", item.Quantity), 9, colQty, y, false)
		writeText(formatPDFMoney(rate), 9, colRate, y, false)
		writeText(formatPDFMoney(item.Total), 9, colPrice, y, false)
		y -= 14
	}

	space(4)
	hline()

	totalsRow := func(label, value string, bold bool, size float64) {
		writeText(label, size, left, y, bold)
		vw := approxWidth(value, size)
		writeText(value, size, right-vw, y, bold)
		y -= size + 6
	}

	if summary.SubTotal > 0 && !summary.CompositeScheme {
		label := "Subtotal"
		if summary.PricesIncludeGST {
			label = "Subtotal (excl. GST)"
		}
		totalsRow(label, formatPDFMoney(summary.SubTotal), false, 9)
	}
	if summary.TaxAmount > 0 && !summary.CompositeScheme {
		totalsRow("GST (5%)", formatPDFMoney(summary.TaxAmount), false, 9)
	}
	if summary.DiscountAmount > 0 {
		totalsRow("Discount", "-"+formatPDFMoney(summary.DiscountAmount), false, 9)
	}
	space(2)
	body.WriteString("0.75 w 0.89 0.91 0.94 RG\n")
	body.WriteString(fmt.Sprintf("%.1f %.1f m %.1f %.1f l S\n", left, y+4, right, y+4))
	body.WriteString("0 g\n")
	totalsRow("Total", formatPDFMoney(summary.Total), true, 12)
	if summary.IsPaid && strings.TrimSpace(summary.PaymentMethod) != "" {
		totalsRow("Payment", strings.ToUpper(summary.PaymentMethod), false, 9)
	}

	space(14)
	centerText("Thank you for dining with us.", 9, false)

	content := body.String()
	contentLen := len(content)

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, 8)

	writeObj := func(id int, payload string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", id, payload)
	}

	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObj(2, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	writeObj(3, fmt.Sprintf(
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Contents 4 0 R /Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> >>",
		pageW, pageH,
	))
	writeObj(4, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", contentLen, content))
	writeObj(5, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	writeObj(6, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")

	xrefPos := out.Len()
	out.WriteString("xref\n")
	fmt.Fprintf(&out, "0 %d\n", len(offsets)+1)
	out.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	out.WriteString("trailer\n")
	fmt.Fprintf(&out, "<< /Size %d /Root 1 0 R >>\n", len(offsets)+1)
	out.WriteString("startxref\n")
	fmt.Fprintf(&out, "%d\n", xrefPos)
	out.WriteString("%%EOF\n")
	return out.Bytes(), nil
}

func billPDFMetaLine(summary BillSummaryView) string {
	parts := []string{}
	if summary.TicketNumber > 0 {
		parts = append(parts, fmt.Sprintf("Ticket #%d", summary.TicketNumber))
	} else {
		parts = append(parts, fmt.Sprintf("Order #%d", summary.OrderNumber))
	}
	if summary.ServiceMode == "takeaway" {
		parts = append(parts, "Takeaway")
	} else if summary.ServiceMode == "eat_here" {
		parts = append(parts, "Eat here")
	}
	if summary.TableNumber != "" && summary.TableNumber != "Counter" && summary.TableNumber != "Takeaway" {
		parts = append(parts, fmt.Sprintf("Table %s", summary.TableNumber))
	}
	return strings.Join(parts, " | ")
}

func formatPDFMoney(amount float64) string {
	return fmt.Sprintf("Rs. %.2f", amount)
}

func pdfEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func truncatePDF(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
