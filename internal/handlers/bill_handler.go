package handlers

import (
	"context"
	"fmt"
	"html"
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
	orderService.StartBillTokenCleanup(context.Background())
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

	filename := fmt.Sprintf("bill-%d.html", summary.OrderNumber)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(buildBillDownloadHTML(*summary)))
}

func billErrorHTML(message string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Bill</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}
.card{background:#fff;padding:32px;border-radius:16px;max-width:360px;text-align:center;box-shadow:0 4px 20px rgba(0,0,0,.08)}</style></head>
<body><div class="card"><h1>Bill</h1><p>%s</p></div></body></html>`, html.EscapeString(message))
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


func buildBillDownloadHTML(summary services.BillSummaryView) string {
	doc := renderCustomerBillDocument(summary)
	doc = strings.Replace(doc, "<!--BILL_BADGE-->", "", 1)
	return strings.Replace(doc, "<!--BILL_ACTIONS-->", "", 1)
}

func renderBillPageHTML(token string, summary services.BillSummaryView) string {
	doc := renderCustomerBillDocument(summary)
	statusBadge := `<span class="badge pending">Review bill — pay at restaurant</span>`
	if summary.IsPaid {
		statusBadge = `<span class="badge paid">Paid</span>`
	}
	doc = strings.Replace(doc, "<!--BILL_BADGE-->", statusBadge, 1)
	actions := renderBillDownloadActions(
		fmt.Sprintf("/b/%s/download", token),
		"Please verify your bill. Payment is collected by restaurant staff. This link expires in 1 hour.",
	)
	return strings.Replace(doc, "<!--BILL_ACTIONS-->", actions, 1)
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
