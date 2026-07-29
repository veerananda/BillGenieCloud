package handlers

import (
	"fmt"
	"strings"

	"restaurant-api/internal/services"
)

func customerBillStylesBlock() string {
	return `<style>
    * { box-sizing: border-box; }
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 0; background: #f8fafc; color: #0f172a; padding: 24px 16px 40px; }
    .page { max-width: 420px; margin: 0 auto; }
    .sheet { background: #fff; border-radius: 18px; overflow: hidden; border: 1px solid #e2e8f0; box-shadow: 0 10px 30px rgba(15,23,42,.08); }
    .head { padding: 24px 20px 18px; text-align: center; background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%); border-bottom: 1px solid #e2e8f0; }
    .brand { font-size: 11px; letter-spacing: .14em; text-transform: uppercase; color: #64748b; font-weight: 700; margin-bottom: 8px; }
    .head h1 { margin: 0; font-size: 1.35rem; line-height: 1.3; }
    .meta, .date, .customer { margin: 6px 0 0; color: #64748b; font-size: .92rem; }
    .badge { display: inline-block; margin-top: 12px; padding: 6px 12px; border-radius: 999px; font-size: .8rem; font-weight: 600; }
    .badge.pending { background: #fef3c7; color: #92400e; }
    .badge.paid { background: #dcfce7; color: #166534; }
    .body { padding: 18px 20px 24px; }
    table { width: 100%; border-collapse: collapse; font-size: .95rem; }
    th { text-align: left; color: #94a3b8; font-size: .72rem; text-transform: uppercase; letter-spacing: .05em; padding-bottom: 10px; border-bottom: 1px solid #e2e8f0; }
    th.qty, th.rate, th.amount, td.qty, td.rate, td.amount { text-align: right; }
    th.qty, td.qty { width: 12%; white-space: nowrap; }
    th.rate, td.rate { width: 18%; white-space: nowrap; padding-left: 8px; }
    th.amount, td.amount { width: 22%; white-space: nowrap; padding-left: 8px; }
    td { padding: 12px 0; border-bottom: 1px solid #f1f5f9; vertical-align: top; }
    .item-name { padding-right: 10px; font-weight: 500; }
    .totals { margin-top: 16px; padding-top: 14px; border-top: 1px solid #e2e8f0; }
    .row { display: flex; justify-content: space-between; gap: 16px; padding: 5px 0; color: #475569; font-size: .95rem; }
    .row.discount { color: #16a34a; }
    .row.total { margin-top: 10px; padding-top: 12px; border-top: 1px solid #e2e8f0; font-size: 1.2rem; font-weight: 800; color: #0f172a; }
    .actions { margin-top: 0; }
    .actions-wrap { margin-top: 20px; }
    .btn { display: flex; width: 100%; align-items: center; justify-content: center; padding: 12px 14px; border-radius: 12px; font-size: .95rem; font-weight: 600; text-decoration: none; }
    .btn-primary { background: #2563eb; color: #fff; }
    .note { margin-top: 16px; text-align: center; color: #94a3b8; font-size: .82rem; line-height: 1.4; }
    .footer { margin-top: 18px; text-align: center; color: #94a3b8; font-size: .85rem; }
    @media print {
      body { background: #fff; padding: 0; }
      .page { max-width: none; }
      .sheet { box-shadow: none; border: none; border-radius: 0; }
      .actions-wrap, .badge, .note, .track-status, .track-footer { display: none !important; }
    }
  </style>`
}

func buildBillMetaLine(summary services.BillSummaryView) string {
	metaParts := []string{}
	if summary.TicketNumber > 0 {
		metaParts = append(metaParts, fmt.Sprintf("Ticket #%d", summary.TicketNumber))
	} else {
		metaParts = append(metaParts, fmt.Sprintf("Order #%d", summary.OrderNumber))
	}
	if summary.ServiceMode == "takeaway" {
		metaParts = append(metaParts, "Takeaway")
	} else if summary.ServiceMode == "eat_here" {
		metaParts = append(metaParts, "Eat here")
	}
	if summary.TableNumber != "" && summary.TableNumber != "Counter" && summary.TableNumber != "Takeaway" {
		metaParts = append(metaParts, fmt.Sprintf("Table %s", escapeBillHTML(summary.TableNumber)))
	}
	return strings.Join(metaParts, " · ")
}

func renderCustomerBillPageFragment(summary services.BillSummaryView) string {
	title := summary.RestaurantName
	if title == "" {
		title = "BillGenie"
	}

	var itemRows strings.Builder
	for _, item := range summary.Items {
		rate := item.UnitRate
		if rate <= 0 && item.Quantity > 0 {
			rate = item.Total / float64(item.Quantity)
		}
		itemRows.WriteString(fmt.Sprintf(
			`<tr><td class="item-name">%s</td><td class="qty">%d</td><td class="rate">%s</td><td class="amount">%s</td></tr>`,
			escapeBillHTML(item.Name), item.Quantity, formatBillCurrency(rate), formatBillCurrency(item.Total),
		))
	}

	subtotalRow := ""
	if summary.SubTotal > 0 && !summary.CompositeScheme {
		subtotalRow = fmt.Sprintf(`<div class="row"><span>%s</span><span>%s</span></div>`,
			subtotalLabelBill(summary.PricesIncludeGST), formatBillCurrency(summary.SubTotal))
	}
	taxRow := ""
	if summary.TaxAmount > 0 && !summary.CompositeScheme {
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

	addressLine := ""
	if strings.TrimSpace(summary.Address) != "" {
		addressLine = fmt.Sprintf(`<p class="meta">%s</p>`, escapeBillHTML(summary.Address))
	}
	contactLine := ""
	if strings.TrimSpace(summary.ContactNumber) != "" {
		contactLine = fmt.Sprintf(`<p class="meta">%s</p>`, escapeBillHTML(summary.ContactNumber))
	}
	gstLine := ""
	if strings.TrimSpace(summary.GstNumber) != "" {
		gstLine = fmt.Sprintf(`<p class="meta">GSTIN: %s</p>`, escapeBillHTML(summary.GstNumber))
	}

	meta := buildBillMetaLine(summary)
	dateLine := formatBillDateTime(summary.CreatedAt)
	customerLine := ""
	if summary.CustomerName != "" && summary.CustomerName != "Guest" &&
		summary.CustomerName != "Takeaway" && summary.CustomerName != "Counter" &&
		summary.CustomerName != "Self Service" {
		customerLine = fmt.Sprintf(`<p class="customer">Customer: %s</p>`, escapeBillHTML(summary.CustomerName))
	}
	if phone := strings.TrimSpace(summary.CustomerPhone); phone != "" {
		customerLine += fmt.Sprintf(`<p class="customer">Phone: %s</p>`, escapeBillHTML(phone))
	}
	attendedLine := ""
	if summary.AttendedByName != "" {
		attendedLine = fmt.Sprintf(`<p class="customer">Attended by: %s</p>`, escapeBillHTML(summary.AttendedByName))
	}

	return fmt.Sprintf(`<div class="page">
    <div class="sheet">
      <div class="head">
        <div class="brand">Bill Summary</div>
        <h1>%s</h1>
        %s
        %s
        %s
        <p class="meta">%s</p>
        %s
        %s
        <!--BILL_BADGE-->
      </div>
      <div class="body">
        <table>
          <thead><tr><th>Item</th><th class="qty">Qty</th><th class="rate">Rate</th><th class="amount">Price</th></tr></thead>
          <tbody>%s</tbody>
        </table>
        <div class="totals">
          %s
          %s
          %s
          <div class="row total"><span>Total</span><span>%s</span></div>
          %s
        </div>
        <p class="footer">Thank you for dining with us.</p>
      </div>
    </div>
    <!--BILL_ACTIONS-->
  </div>`,
		escapeBillHTML(title),
		addressLine,
		contactLine,
		gstLine,
		meta,
		dateLineHTML(dateLine),
		customerLine+attendedLine,
		itemRows.String(),
		subtotalRow,
		taxRow,
		discountRow,
		formatBillCurrency(summary.Total),
		paymentRow,
	)
}

func renderBillDownloadActions(downloadHref, note string) string {
	return fmt.Sprintf(`<div class="actions-wrap"><div class="actions"><a class="btn btn-primary" href="%s">Download bill</a></div><p class="note">%s</p></div>`,
		escapeBillHTML(downloadHref), escapeBillHTML(note))
}

func renderCustomerBillDocument(summary services.BillSummaryView) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Bill #%d</title>
  %s
</head>
<body>
%s
</body>
</html>`, summary.OrderNumber, customerBillStylesBlock(), renderCustomerBillPageFragment(summary))
}
