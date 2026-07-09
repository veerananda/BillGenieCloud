package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupBillRoutes registers public customer bill pages (no auth).
func SetupBillRoutes(router *gin.Engine, db *gorm.DB) {
	orderService := services.NewOrderService(db)
	handler := &BillHandler{orderService: orderService}

	router.GET("/b/:token", handler.BillPage)
	router.GET("/b/:token/download", handler.BillDownload)

	log.Println("✅ Customer bill routes registered at /b/:token")
}

type BillHandler struct {
	orderService *services.OrderService
}

func (h *BillHandler) loadBill(token string) (*services.BillSummaryView, int, string) {
	order, restaurant, err := h.orderService.GetOrderByBillToken(token)
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			return nil, http.StatusGone, "This bill link has expired."
		}
		return nil, http.StatusNotFound, "Bill not found."
	}

	summary := services.BuildBillSummary(order, restaurant)
	return &summary, http.StatusOK, ""
}

func (h *BillHandler) BillPage(c *gin.Context) {
	token := c.Param("token")
	summary, code, message := h.loadBill(token)
	if summary == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(billErrorHTML(message)))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(renderBillPageHTML(token, *summary)))
}

func (h *BillHandler) BillDownload(c *gin.Context) {
	token := c.Param("token")
	summary, code, message := h.loadBill(token)
	if summary == nil {
		c.Data(code, "text/html; charset=utf-8", []byte(billErrorHTML(message)))
		return
	}

	filename := fmt.Sprintf("bill-%d.txt", summary.OrderNumber)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(buildBillReceiptText(*summary)))
}

func billErrorHTML(message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Bill</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}
.card{background:#fff;padding:32px;border-radius:16px;max-width:360px;text-align:center;box-shadow:0 4px 20px rgba(0,0,0,.08)}</style></head>
<body><div class="card"><h1>Bill</h1><p>%s</p></div></body></html>`, message)
}

func formatBillCurrency(amount float64) string {
	return fmt.Sprintf("₹%.2f", amount)
}

func formatBillDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.Local).Format("02 Jan 2006, 03:04 PM")
}

func subtotalLabelBill(pricesIncludeGST bool) string {
	if pricesIncludeGST {
		return "Taxable value"
	}
	return "Subtotal"
}

func buildBillReceiptText(summary services.BillSummaryView) string {
	var lines []string
	divider := "--------------------------------"

	if summary.RestaurantName != "" {
		lines = append(lines, summary.RestaurantName)
	}
	if summary.Address != "" {
		lines = append(lines, summary.Address)
	}
	if summary.ContactNumber != "" {
		lines = append(lines, summary.ContactNumber)
	}
	lines = append(lines, "")
	lines = append(lines, "BILL SUMMARY")
	lines = append(lines, divider)
	lines = append(lines, fmt.Sprintf("Order: #%d", summary.OrderNumber))
	if summary.TableNumber != "" {
		lines = append(lines, fmt.Sprintf("Table: %s", summary.TableNumber))
	}
	if summary.CustomerName != "" && summary.CustomerName != "Guest" {
		lines = append(lines, fmt.Sprintf("Customer: %s", summary.CustomerName))
	}
	if !summary.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Date: %s", formatBillDateTime(summary.CreatedAt)))
	}
	lines = append(lines, divider)
	lines = append(lines, "Items")
	for _, item := range summary.Items {
		lines = append(lines, fmt.Sprintf("%d x %s", item.Quantity, item.Name))
		lines = append(lines, fmt.Sprintf("   %s", formatBillCurrency(item.Total)))
	}
	lines = append(lines, divider)
	if summary.SubTotal > 0 {
		lines = append(lines, fmt.Sprintf("%s: %s", subtotalLabelBill(summary.PricesIncludeGST), formatBillCurrency(summary.SubTotal)))
	}
	if summary.TaxAmount > 0 {
		lines = append(lines, fmt.Sprintf("GST (5%%): %s", formatBillCurrency(summary.TaxAmount)))
	}
	if summary.DiscountAmount > 0 {
		lines = append(lines, fmt.Sprintf("Discount: -%s", formatBillCurrency(summary.DiscountAmount)))
	}
	lines = append(lines, fmt.Sprintf("Total: %s", formatBillCurrency(summary.Total)))
	if summary.IsPaid && summary.PaymentMethod != "" {
		lines = append(lines, fmt.Sprintf("Payment: %s", strings.ToUpper(summary.PaymentMethod)))
	} else {
		lines = append(lines, "Payment: Pending at restaurant")
	}
	lines = append(lines, divider)
	lines = append(lines, "Thank you!")
	return strings.Join(lines, "\n")
}

func renderBillPageHTML(token string, summary services.BillSummaryView) string {
	title := summary.RestaurantName
	if title == "" {
		title = "BillGenie"
	}

	statusBadge := `<span class="badge pending">Review bill — pay at restaurant</span>`
	if summary.IsPaid {
		statusBadge = `<span class="badge paid">Paid</span>`
	}

	var itemRows strings.Builder
	for _, item := range summary.Items {
		itemRows.WriteString(fmt.Sprintf(
			`<tr><td class="item-name">%s</td><td class="qty">%d</td><td class="amount">%s</td></tr>`,
			escapeBillHTML(item.Name), item.Quantity, formatBillCurrency(item.Total),
		))
	}

	subtotalRow := ""
	if summary.SubTotal > 0 {
		subtotalRow = fmt.Sprintf(`<div class="row"><span>%s</span><span>%s</span></div>`,
			subtotalLabelBill(summary.PricesIncludeGST), formatBillCurrency(summary.SubTotal))
	}
	taxRow := ""
	if summary.TaxAmount > 0 {
		taxRow = fmt.Sprintf(`<div class="row"><span>GST (5%%)</span><span>%s</span></div>`,
			formatBillCurrency(summary.TaxAmount))
	}
	discountRow := ""
	if summary.DiscountAmount > 0 {
		discountRow = fmt.Sprintf(`<div class="row discount"><span>Discount</span><span>-%s</span></div>`,
			formatBillCurrency(summary.DiscountAmount))
	}
	paymentRow := ""
	if summary.IsPaid && summary.PaymentMethod != "" {
		paymentRow = fmt.Sprintf(`<div class="row"><span>Payment</span><span>%s</span></div>`,
			strings.ToUpper(escapeBillHTML(summary.PaymentMethod)))
	}

	meta := fmt.Sprintf("Order #%d", summary.OrderNumber)
	if summary.TableNumber != "" {
		meta += fmt.Sprintf(" · Table %s", escapeBillHTML(summary.TableNumber))
	}
	dateLine := formatBillDateTime(summary.CreatedAt)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Bill #%d</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, sans-serif; margin: 0; background: #f8fafc; color: #0f172a; }
    .wrap { max-width: 480px; margin: 0 auto; padding: 20px 16px 40px; }
    .card { background: #fff; border-radius: 16px; box-shadow: 0 4px 24px rgba(15,23,42,.08); overflow: hidden; }
    .head { padding: 20px 20px 12px; text-align: center; border-bottom: 1px solid #e2e8f0; }
    .head h1 { margin: 0; font-size: 1.25rem; }
    .head p { margin: 6px 0 0; color: #64748b; font-size: .9rem; }
    .badge { display: inline-block; margin-top: 12px; padding: 6px 12px; border-radius: 999px; font-size: .8rem; font-weight: 600; }
    .badge.pending { background: #fef3c7; color: #92400e; }
    .badge.paid { background: #dcfce7; color: #166534; }
    .body { padding: 16px 20px 20px; }
    table { width: 100%%; border-collapse: collapse; font-size: .95rem; }
    th { text-align: left; color: #94a3b8; font-size: .75rem; text-transform: uppercase; letter-spacing: .04em; padding-bottom: 8px; }
    th.qty, th.amount, td.qty, td.amount { text-align: right; }
    td { padding: 10px 0; border-bottom: 1px solid #f1f5f9; vertical-align: top; }
    .item-name { padding-right: 8px; }
    .totals { margin-top: 16px; padding-top: 12px; border-top: 1px solid #e2e8f0; }
    .row { display: flex; justify-content: space-between; padding: 4px 0; color: #475569; font-size: .95rem; }
    .row.discount { color: #16a34a; }
    .row.total { margin-top: 8px; padding-top: 10px; border-top: 1px solid #e2e8f0; font-size: 1.15rem; font-weight: 700; color: #0f172a; }
    .actions { display: flex; gap: 10px; margin-top: 20px; }
  .btn { flex: 1; display: inline-flex; align-items: center; justify-content: center; padding: 12px 14px; border-radius: 12px; font-size: .95rem; font-weight: 600; text-decoration: none; border: none; cursor: pointer; }
    .btn-primary { background: #2563eb; color: #fff; }
    .btn-secondary { background: #f1f5f9; color: #0f172a; }
    .note { margin-top: 16px; text-align: center; color: #94a3b8; font-size: .82rem; line-height: 1.4; }
    @media print { body { background: #fff; } .actions, .note { display: none; } .wrap { padding: 0; } .card { box-shadow: none; } }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="head">
        <h1>%s</h1>
        <p>%s</p>
        %s
        %s
      </div>
      <div class="body">
        <table>
          <thead><tr><th>Item</th><th class="qty">Qty</th><th class="amount">Amount</th></tr></thead>
          <tbody>%s</tbody>
        </table>
        <div class="totals">
          %s
          %s
          %s
          <div class="row total"><span>Total</span><span>%s</span></div>
          %s
        </div>
        <div class="actions">
          <a class="btn btn-primary" href="/b/%s/download">Download bill</a>
          <button class="btn btn-secondary" type="button" onclick="window.print()">Print / Save PDF</button>
        </div>
        <p class="note">Please verify your bill. Payment is collected by restaurant staff.</p>
      </div>
    </div>
  </div>
</body>
</html>`,
		summary.OrderNumber,
		escapeBillHTML(title),
		meta,
		statusBadge,
		dateLineHTML(dateLine),
		itemRows.String(),
		subtotalRow,
		taxRow,
		discountRow,
		formatBillCurrency(summary.Total),
		paymentRow,
		token,
	)
}

func dateLineHTML(dateLine string) string {
	if dateLine == "" {
		return ""
	}
	return fmt.Sprintf(`<p>%s</p>`, escapeBillHTML(dateLine))
}

func escapeBillHTML(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	return value
}
