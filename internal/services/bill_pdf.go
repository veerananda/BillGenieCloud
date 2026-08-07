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
	textWidth := func(text string, size float64, bold bool) float64 {
		return helveticaStringWidth(text, size, bold)
	}
	centerText := func(text string, size float64, bold bool) {
		w := textWidth(text, size, bold)
		x := left + (contentW-w)/2
		if x < left {
			x = left
		}
		writeText(text, size, x, y, bold)
		y -= size + 5
	}
	rightText := func(text string, size, rightEdge float64, bold bool) {
		w := textWidth(text, size, bold)
		writeText(text, size, rightEdge-w, y, bold)
	}
	hline := func() {
		body.WriteString("0.6 w 0.82 0.86 0.90 RG\n")
		body.WriteString(fmt.Sprintf("%.1f %.1f m %.1f %.1f l S\n", left, y, right, y))
		body.WriteString("0 g\n")
		y -= 10
	}
	space := func(dy float64) { y -= dy }

	centerText("BILL SUMMARY", 8, true)
	space(4)
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

	space(10)
	hline()

	// Column right-edges: give RATE and PRICE a clear gap between values.
	const colQtyRight = left + 168
	const colRateRight = left + 228
	const colPriceRight = right

	writeText("ITEM", 7, left, y, true)
	rightText("QTY", 7, colQtyRight, true)
	rightText("RATE", 7, colRateRight, true)
	rightText("PRICE", 7, colPriceRight, true)
	y -= 12
	hline()

	for _, item := range summary.Items {
		if y < 100 {
			break
		}
		rate := item.UnitRate
		if rate <= 0 && item.Quantity > 0 {
			rate = item.Total / float64(item.Quantity)
		}
		name := truncatePDF(item.Name, 26)
		writeText(name, 9, left, y, false)
		rightText(fmt.Sprintf("%d", item.Quantity), 9, colQtyRight, false)
		rightText(formatPDFMoney(rate), 9, colRateRight, false)
		rightText(formatPDFMoney(item.Total), 9, colPriceRight, false)
		y -= 14
	}

	space(6)
	hline()

	totalsRow := func(label, value string, bold bool, size float64) {
		writeText(label, size, left, y, bold)
		rightText(value, size, right, bold)
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

	// Divider above Total — leave clear gap so the line does not cross the text.
	space(6)
	body.WriteString("0.7 w 0.78 0.82 0.86 RG\n")
	body.WriteString(fmt.Sprintf("%.1f %.1f m %.1f %.1f l S\n", left, y, right, y))
	body.WriteString("0 g\n")
	space(12)
	totalsRow("Total", formatPDFMoney(summary.Total), true, 12)
	if summary.IsPaid && strings.TrimSpace(summary.PaymentMethod) != "" {
		space(2)
		totalsRow("Payment", strings.ToUpper(summary.PaymentMethod), false, 9)
	}

	space(16)
	centerText("Thank you for dining with us.", 9, false)
	space(8)
	centerText("Powered by BillGenie", 8, false)

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

// helveticaStringWidth returns approximate rendered width using Adobe AFM glyph widths.
func helveticaStringWidth(s string, size float64, bold bool) float64 {
	widths := helveticaWidths
	if bold {
		widths = helveticaBoldWidths
	}
	var total float64
	for _, r := range s {
		if r > 255 {
			total += 600
			continue
		}
		w := widths[byte(r)]
		if w == 0 {
			w = 600
		}
		total += float64(w)
	}
	return total * size / 1000
}

// Standard Helvetica AFM widths (1/1000 em) for ASCII.
var helveticaWidths = [256]uint16{
	32: 278, 33: 278, 34: 355, 35: 556, 36: 556, 37: 889, 38: 667, 39: 191,
	40: 333, 41: 333, 42: 389, 43: 584, 44: 278, 45: 333, 46: 278, 47: 278,
	48: 556, 49: 556, 50: 556, 51: 556, 52: 556, 53: 556, 54: 556, 55: 556,
	56: 556, 57: 556, 58: 278, 59: 278, 60: 584, 61: 584, 62: 584, 63: 556,
	64: 1015, 65: 667, 66: 667, 67: 722, 68: 722, 69: 667, 70: 611, 71: 778,
	72: 722, 73: 278, 74: 500, 75: 667, 76: 556, 77: 833, 78: 722, 79: 778,
	80: 667, 81: 778, 82: 722, 83: 667, 84: 611, 85: 722, 86: 667, 87: 944,
	88: 667, 89: 667, 90: 611, 91: 278, 92: 278, 93: 278, 94: 469, 95: 556,
	96: 333, 97: 556, 98: 556, 99: 500, 100: 556, 101: 556, 102: 278, 103: 556,
	104: 556, 105: 222, 106: 222, 107: 500, 108: 222, 109: 833, 110: 556, 111: 556,
	112: 556, 113: 556, 114: 333, 115: 500, 116: 278, 117: 556, 118: 500, 119: 722,
	120: 500, 121: 500, 122: 500, 123: 334, 124: 260, 125: 334, 126: 584,
}

// Standard Helvetica-Bold AFM widths (1/1000 em) for ASCII.
var helveticaBoldWidths = [256]uint16{
	32: 278, 33: 333, 34: 474, 35: 556, 36: 556, 37: 889, 38: 722, 39: 238,
	40: 333, 41: 333, 42: 389, 43: 584, 44: 278, 45: 333, 46: 278, 47: 278,
	48: 556, 49: 556, 50: 556, 51: 556, 52: 556, 53: 556, 54: 556, 55: 556,
	56: 556, 57: 556, 58: 333, 59: 333, 60: 584, 61: 584, 62: 584, 63: 611,
	64: 975, 65: 722, 66: 722, 67: 722, 68: 722, 69: 667, 70: 611, 71: 778,
	72: 722, 73: 278, 74: 556, 75: 722, 76: 611, 77: 833, 78: 722, 79: 778,
	80: 667, 81: 778, 82: 722, 83: 667, 84: 611, 85: 722, 86: 667, 87: 944,
	88: 667, 89: 667, 90: 611, 91: 333, 92: 278, 93: 333, 94: 584, 95: 556,
	96: 333, 97: 556, 98: 611, 99: 556, 100: 611, 101: 556, 102: 333, 103: 611,
	104: 611, 105: 278, 106: 278, 107: 556, 108: 278, 109: 889, 110: 611, 111: 611,
	112: 611, 113: 611, 114: 389, 115: 556, 116: 333, 117: 611, 118: 556, 119: 778,
	120: 556, 121: 556, 122: 500, 123: 389, 124: 280, 125: 389, 126: 584,
}
