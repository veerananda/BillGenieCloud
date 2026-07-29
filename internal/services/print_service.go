package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PrintJobTypeKOT  = "kot"
	PrintJobTypeBill = "bill"
	PrintTargetKOT   = "kot_printer"
	PrintTargetBill  = "bill_printer"
	PrintStatusPending = "pending"
	PrintStatusClaimed = "claimed"
	PrintStatusDone    = "done"
	PrintStatusFailed  = "failed"
)

type PrintService struct {
	db *gorm.DB
}

func NewPrintService(db *gorm.DB) *PrintService {
	return &PrintService{db: db}
}

func (s *PrintService) GetOrCreateSettings(restaurantID string) (*models.RestaurantPrintSettings, error) {
	var settings models.RestaurantPrintSettings
	err := s.db.Where("restaurant_id = ?", restaurantID).First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	settings = models.RestaurantPrintSettings{
		RestaurantID:     restaurantID,
		BillPrinterPort:  9100,
		KotPrinterPort:   9100,
		BillPaperWidthMm: 58,
		KotPaperWidthMm:  58,
		TopFeedLines:     0,
		BottomFeedLines:  3,
	}
	if err := s.db.Create(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

type UpdatePrintSettingsInput struct {
	BillPrinterHost     *string `json:"bill_printer_host"`
	BillPrinterPort     *int    `json:"bill_printer_port"`
	KotPrinterHost      *string `json:"kot_printer_host"`
	KotPrinterPort      *int    `json:"kot_printer_port"`
	BillPrintingEnabled     *bool   `json:"bill_printing_enabled"`
	KotPrintingEnabled      *bool   `json:"kot_printing_enabled"`
	BillAutoPrintOnCheckout *bool   `json:"bill_auto_print_on_checkout"`
	BillPaperWidthMm        *int    `json:"bill_paper_width_mm"`
	KotPaperWidthMm         *int    `json:"kot_paper_width_mm"`
	TopFeedLines            *int    `json:"top_feed_lines"`
	BottomFeedLines         *int    `json:"bottom_feed_lines"`
}

func clampFeedLines(n int) int {
	if n < 0 {
		return 0
	}
	if n > 20 {
		return 20
	}
	return n
}

func normalizePaperWidthMm(n int) int {
	if n == 80 {
		return 80
	}
	return 58
}

func thermalColsForPaper(paperWidthMm int) int {
	if normalizePaperWidthMm(paperWidthMm) == 80 {
		return 48
	}
	return 32
}

func (s *PrintService) UpdateSettings(restaurantID string, input UpdatePrintSettingsInput) (*models.RestaurantPrintSettings, error) {
	settings, err := s.GetOrCreateSettings(restaurantID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if input.BillPrinterHost != nil {
		updates["bill_printer_host"] = strings.TrimSpace(*input.BillPrinterHost)
	}
	if input.BillPrinterPort != nil {
		port := *input.BillPrinterPort
		if port <= 0 || port > 65535 {
			port = 9100
		}
		updates["bill_printer_port"] = port
	}
	if input.KotPrinterHost != nil {
		updates["kot_printer_host"] = strings.TrimSpace(*input.KotPrinterHost)
	}
	if input.KotPrinterPort != nil {
		port := *input.KotPrinterPort
		if port <= 0 || port > 65535 {
			port = 9100
		}
		updates["kot_printer_port"] = port
	}
	if input.BillPrintingEnabled != nil {
		updates["bill_printing_enabled"] = *input.BillPrintingEnabled
	}
	if input.KotPrintingEnabled != nil {
		updates["kot_printing_enabled"] = *input.KotPrintingEnabled
	}
	if input.BillAutoPrintOnCheckout != nil {
		updates["bill_auto_print_on_checkout"] = *input.BillAutoPrintOnCheckout
	}
	if input.BillPaperWidthMm != nil {
		updates["bill_paper_width_mm"] = normalizePaperWidthMm(*input.BillPaperWidthMm)
	}
	if input.KotPaperWidthMm != nil {
		updates["kot_paper_width_mm"] = normalizePaperWidthMm(*input.KotPaperWidthMm)
	}
	if input.TopFeedLines != nil {
		updates["top_feed_lines"] = clampFeedLines(*input.TopFeedLines)
	}
	if input.BottomFeedLines != nil {
		updates["bottom_feed_lines"] = clampFeedLines(*input.BottomFeedLines)
	}
	if len(updates) == 0 {
		return settings, nil
	}
	if err := s.db.Model(settings).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetOrCreateSettings(restaurantID)
}

// RotateAgentAPIKey generates a new agent key; plaintext is returned once.
func (s *PrintService) RotateAgentAPIKey(restaurantID string) (plaintext string, settings *models.RestaurantPrintSettings, err error) {
	settings, err = s.GetOrCreateSettings(restaurantID)
	if err != nil {
		return "", nil, err
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	plaintext = "bgpa_" + hex.EncodeToString(buf)
	hint := plaintext
	if len(hint) > 4 {
		hint = hint[len(hint)-4:]
	}
	if err := s.db.Model(settings).Updates(map[string]interface{}{
		"agent_api_key_hash": hashSecret(plaintext),
		"agent_api_key_hint": hint,
	}).Error; err != nil {
		return "", nil, err
	}
	settings.AgentAPIKeyHint = hint
	return plaintext, settings, nil
}

func (s *PrintService) FindRestaurantIDByAgentKey(rawKey string) (string, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return "", fmt.Errorf("missing agent key")
	}
	hashed := hashSecret(rawKey)
	var settings models.RestaurantPrintSettings
	if err := s.db.Where("agent_api_key_hash = ?", hashed).First(&settings).Error; err != nil {
		return "", fmt.Errorf("invalid agent key")
	}
	return settings.RestaurantID, nil
}

// EnqueueKOTForOrder queues a kitchen slip when KOT printing is enabled.
// preferLatestBatch is kept for callers (create vs update); the ADD-ON label is
// derived from whether an earlier KOT batch already exists on the order.
func (s *PrintService) EnqueueKOTForOrder(order *models.Order, preferLatestBatch bool) {
	if order == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("print enqueue KOT panic: %v", r)
			}
		}()
		if err := s.enqueueKOT(order, preferLatestBatch); err != nil {
			log.Printf("print enqueue KOT failed for order %s: %v", order.ID, err)
		}
	}()
}

func (s *PrintService) enqueueKOT(order *models.Order, preferLatestBatch bool) error {
	settings, err := s.GetOrCreateSettings(order.RestaurantID)
	if err != nil {
		return err
	}
	if !settings.KotPrintingEnabled {
		return nil
	}
	if strings.TrimSpace(settings.KotPrinterHost) == "" {
		log.Printf(
			"print enqueue KOT skipped for order %s: kot printing enabled but kot_printer_host is empty (browser KOT may still print)",
			order.ID,
		)
		return nil
	}

	full, err := s.loadOrderForPrint(order.ID)
	if err != nil {
		return err
	}
	order = full

	items, isAddOn := selectKOTSlipItems(order.Items, preferLatestBatch)
	if len(items) == 0 {
		return nil
	}

	var restaurant models.Restaurant
	_ = s.db.Select("name", "category_display_blocklist").Where("id = ?", order.RestaurantID).First(&restaurant).Error

	text := buildKOTPayload(restaurant, order, items, isAddOn)
	job := models.PrintJob{
		RestaurantID: order.RestaurantID,
		OrderID:      order.ID,
		JobType:      PrintJobTypeKOT,
		Target:       PrintTargetKOT,
		PayloadText:  text,
		Status:       PrintStatusPending,
	}
	return s.db.Create(&job).Error
}

// selectKOTSlipItems picks lines for the slip and whether to title it ADD-ON.
// ADD-ON only when a prior non-cancelled kitchen batch (different sub_id) already exists.
func selectKOTSlipItems(all []models.OrderItem, preferLatestBatch bool) (items []models.OrderItem, isAddOn bool) {
	latestSub := ""
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Status == "cancelled" {
			continue
		}
		if all[i].SubId != "" {
			latestSub = all[i].SubId
			break
		}
	}

	priorBatchExists := false
	for _, it := range all {
		if it.Status == "cancelled" {
			continue
		}
		if latestSub != "" && preferLatestBatch {
			if it.SubId == latestSub {
				items = append(items, it)
			} else {
				priorBatchExists = true
			}
			continue
		}
		items = append(items, it)
	}

	if preferLatestBatch && latestSub != "" {
		return items, priorBatchExists
	}

	// New order create: every active line shares one batch → never ADD-ON.
	if latestSub == "" {
		return items, false
	}
	for _, it := range all {
		if it.Status == "cancelled" {
			continue
		}
		if it.SubId != "" && it.SubId != latestSub {
			return items, true
		}
	}
	return items, false
}

// EnqueueBillForOrder queues a customer bill when bill printing is enabled.
// Returns whether a print job was actually created (false = disabled / no host / empty).
func (s *PrintService) EnqueueBillForOrder(order *models.Order) (queued bool, err error) {
	if order == nil {
		return false, nil
	}
	queued, err = s.enqueueBill(order)
	if err != nil {
		log.Printf("print enqueue bill failed for order %s: %v", order.ID, err)
	}
	return queued, err
}

func (s *PrintService) enqueueBill(order *models.Order) (bool, error) {
	settings, err := s.GetOrCreateSettings(order.RestaurantID)
	if err != nil {
		return false, err
	}
	billHost := strings.TrimSpace(settings.BillPrinterHost)
	kotHost := strings.TrimSpace(settings.KotPrinterHost)
	// Single physical WiFi printer is often configured only under KOT host.
	if !settings.BillPrintingEnabled || (billHost == "" && kotHost == "") {
		return false, nil
	}

	full, err := s.loadOrderForPrint(order.ID)
	if err != nil {
		return false, err
	}
	order = full

	var restaurant models.Restaurant
	_ = s.db.Select("name", "address", "contact_number", "phone", "gst_number", "category_display_blocklist").
		Where("id = ?", order.RestaurantID).First(&restaurant).Error

	active := make([]models.OrderItem, 0, len(order.Items))
	for _, it := range order.Items {
		if it.Status != "cancelled" {
			active = append(active, it)
		}
	}
	if len(active) == 0 {
		return false, nil
	}

	text := buildBillPayload(restaurant, order, active, settings.BillPaperWidthMm)
	job := models.PrintJob{
		RestaurantID: order.RestaurantID,
		OrderID:      order.ID,
		JobType:      PrintJobTypeBill,
		Target:       PrintTargetBill,
		PayloadText:  text,
		Status:       PrintStatusPending,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return false, err
	}
	return true, nil
}

// EnqueueTestPrint queues a short slip so staff can verify Wi‑Fi/LAN agent printing.
// Target is "kot" or "bill". Returns queued=false when no printer host resolves.
func (s *PrintService) EnqueueTestPrint(restaurantID, target string) (bool, error) {
	settings, err := s.GetOrCreateSettings(restaurantID)
	if err != nil {
		return false, err
	}

	var jobType, printTarget string
	var paperWidthMm int
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "kot":
		jobType = PrintJobTypeKOT
		printTarget = PrintTargetKOT
		paperWidthMm = settings.KotPaperWidthMm
	case "bill":
		jobType = PrintJobTypeBill
		printTarget = PrintTargetBill
		paperWidthMm = settings.BillPaperWidthMm
	default:
		return false, fmt.Errorf("target must be kot or bill")
	}

	host, _ := resolvePrinter(settings, printTarget)
	if host == "" {
		return false, nil
	}

	var restaurant models.Restaurant
	_ = s.db.Select("name").Where("id = ?", restaurantID).First(&restaurant).Error

	text := buildTestPrintPayload(restaurant.Name, target, host, paperWidthMm)
	job := models.PrintJob{
		RestaurantID: restaurantID,
		OrderID:      "",
		JobType:      jobType,
		Target:       printTarget,
		PayloadText:  text,
		Status:       PrintStatusPending,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return false, err
	}
	return true, nil
}

func buildTestPrintPayload(restaurantName, target, host string, paperWidthMm int) string {
	width := thermalColsForPaper(paperWidthMm)
	divider := strings.Repeat("-", width)
	label := "KOT"
	if strings.EqualFold(target, "bill") {
		label = "BILL"
	}
	var b strings.Builder
	if name := strings.TrimSpace(restaurantName); name != "" {
		b.WriteString(printCenterLine(name, width))
		b.WriteByte('\n')
	}
	b.WriteString(printCenterLine("TEST PRINT", width))
	b.WriteByte('\n')
	b.WriteString(printCenterLine(label+" Wi-Fi / LAN", width))
	b.WriteByte('\n')
	b.WriteString(divider)
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("Host: %s\n", host))
	b.WriteString(time.Now().Format("02 Jan 2006 03:04 PM"))
	b.WriteByte('\n')
	b.WriteString(divider)
	b.WriteByte('\n')
	for _, line := range wrapPrintWords("If you can read this, the print agent can reach this printer.", width) {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString(divider)
	b.WriteByte('\n')
	b.WriteString(printCenterLine("BillGenie", width))
	b.WriteByte('\n')
	return b.String()
}

func (s *PrintService) loadOrderForPrint(orderID string) (*models.Order, error) {
	var order models.Order
	err := s.db.Preload("Items.MenuItem").Where("id = ?", orderID).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// LoadOrderForEnqueue loads an order scoped to a restaurant for manual bill print.
func (s *PrintService) LoadOrderForEnqueue(restaurantID, orderID string) (*models.Order, error) {
	var order models.Order
	err := s.db.Preload("Items.MenuItem").
		Where("id = ? AND restaurant_id = ?", orderID, restaurantID).
		First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

type AgentJobView struct {
	ID              string `json:"id"`
	JobType         string `json:"job_type"`
	Target          string `json:"target"`
	PayloadText     string `json:"payload_text"`
	PrinterHost     string `json:"printer_host"`
	PrinterPort     int    `json:"printer_port"`
	TopFeedLines    int    `json:"top_feed_lines"`
	BottomFeedLines int    `json:"bottom_feed_lines"`
	CreatedAt       string `json:"created_at"`
}

func (s *PrintService) ClaimPendingJobs(restaurantID, agentID string, limit int) ([]AgentJobView, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	settings, err := s.GetOrCreateSettings(restaurantID)
	if err != nil {
		return nil, err
	}

	var claimed []AgentJobView
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var jobs []models.PrintJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("restaurant_id = ? AND status = ?", restaurantID, PrintStatusPending).
			Order("created_at ASC").
			Limit(limit).
			Find(&jobs).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, job := range jobs {
			host, port := resolvePrinter(settings, job.Target)
			if host == "" {
				_ = tx.Model(&job).Updates(map[string]interface{}{
					"status":        PrintStatusFailed,
					"error_message": "printer host not configured",
					"completed_at":  now,
					"attempts":      job.Attempts + 1,
				}).Error
				continue
			}
			updates := map[string]interface{}{
				"status":     PrintStatusClaimed,
				"claimed_by": agentID,
				"claimed_at": now,
				"attempts":   job.Attempts + 1,
			}
			if err := tx.Model(&job).Updates(updates).Error; err != nil {
				return err
			}
			claimed = append(claimed, AgentJobView{
				ID:              job.ID,
				JobType:         job.JobType,
				Target:          job.Target,
				PayloadText:     job.PayloadText,
				PrinterHost:     host,
				PrinterPort:     port,
				TopFeedLines:    settings.TopFeedLines,
				BottomFeedLines: settings.BottomFeedLines,
				CreatedAt:       job.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil
	})
	return claimed, err
}

func resolvePrinter(settings *models.RestaurantPrintSettings, target string) (string, int) {
	if target == PrintTargetKOT {
		port := settings.KotPrinterPort
		if port <= 0 {
			port = 9100
		}
		return strings.TrimSpace(settings.KotPrinterHost), port
	}
	host := strings.TrimSpace(settings.BillPrinterHost)
	port := settings.BillPrinterPort
	if host == "" {
		// Same WiFi/LAN printer used for KOT
		host = strings.TrimSpace(settings.KotPrinterHost)
		port = settings.KotPrinterPort
	}
	if port <= 0 {
		port = 9100
	}
	return host, port
}

func (s *PrintService) CompleteJob(restaurantID, jobID string, failed bool, errMsg string) error {
	var job models.PrintJob
	if err := s.db.Where("id = ? AND restaurant_id = ?", jobID, restaurantID).First(&job).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	status := PrintStatusDone
	if failed {
		status = PrintStatusFailed
	}
	return s.db.Model(&job).Updates(map[string]interface{}{
		"status":        status,
		"completed_at":  now,
		"error_message": errMsg,
	}).Error
}

// printItemNameAndCategory returns the menu item name and its category for slips.
func printItemNameAndCategory(it models.OrderItem) (name, category string) {
	if it.MenuItem != nil {
		name = strings.TrimSpace(it.MenuItem.Name)
		category = strings.TrimSpace(it.MenuItem.Category)
	}
	if name == "" {
		name = "Item"
	}
	return name, category
}

func printItemDisplayName(it models.OrderItem, extraBlocklist []string) string {
	name, category := printItemNameAndCategory(it)
	return FormatItemDisplayName(name, category, it.VariantLabel, extraBlocklist)
}

func buildKOTPayload(restaurant models.Restaurant, order *models.Order, items []models.OrderItem, isAddOn bool) string {
	var b strings.Builder
	divider := "--------------------------------"
	if restaurant.Name != "" {
		b.WriteString(restaurant.Name)
		b.WriteByte('\n')
	}
	if isAddOn {
		b.WriteString("KOT (ADD-ON)\n")
	} else {
		b.WriteString("KOT\n")
	}
	b.WriteString(divider)
	b.WriteByte('\n')
	num := order.TicketNumber
	if num == 0 {
		num = order.OrderNumber
	}
	if num > 0 {
		b.WriteString(fmt.Sprintf("#%d\n", num))
	}
	if order.OrderType == "counter" {
		mode := order.ServiceMode
		if mode == "" {
			mode = "eat_here"
		}
		b.WriteString(fmt.Sprintf("Counter · %s\n", mode))
	} else if order.TableNumber != "" {
		b.WriteString(fmt.Sprintf("Table: %s\n", order.TableNumber))
	}
	b.WriteString(time.Now().Format("02 Jan 3:04 PM"))
	b.WriteByte('\n')
	b.WriteString(divider)
	b.WriteByte('\n')
	blocklist := ParseCategoryDisplayBlocklist(restaurant.CategoryDisplayBlocklist)
	for _, it := range items {
		name := printItemDisplayName(it, blocklist)
		b.WriteString(fmt.Sprintf("%d x %s\n", it.Quantity, name))
		if notes := strings.TrimSpace(it.Notes); notes != "" {
			b.WriteString(fmt.Sprintf("   * %s\n", notes))
		}
	}
	b.WriteString(divider)
	b.WriteByte('\n')
	return b.String()
}

func buildBillPayload(restaurant models.Restaurant, order *models.Order, items []models.OrderItem, paperWidthMm int) string {
	width := thermalColsForPaper(paperWidthMm)
	var b strings.Builder
	divider := strings.Repeat("-", width)
	if restaurant.Name != "" {
		b.WriteString(printCenterLine(restaurant.Name, width))
		b.WriteByte('\n')
	}
	if restaurant.Address != "" {
		for _, line := range wrapPrintWords(restaurant.Address, width) {
			b.WriteString(printCenterLine(line, width))
			b.WriteByte('\n')
		}
	}
	contact := restaurant.ContactNumber
	if contact == "" {
		contact = restaurant.Phone
	}
	if contact != "" {
		b.WriteString(printCenterLine("Ph: "+contact, width))
		b.WriteByte('\n')
	}
	if gst := strings.TrimSpace(restaurant.GstNumber); gst != "" {
		b.WriteString(printCenterLine("GSTIN: "+gst, width))
		b.WriteByte('\n')
	}
	b.WriteString(printCenterLine("BILL", width))
	b.WriteByte('\n')
	b.WriteString(divider)
	b.WriteByte('\n')
	num := order.TicketNumber
	if num == 0 {
		num = order.OrderNumber
	}
	if num > 0 {
		b.WriteString(fmt.Sprintf("Order: #%d\n", num))
	}
	if order.TableNumber != "" && order.OrderType != "counter" {
		b.WriteString(fmt.Sprintf("Table: %s\n", order.TableNumber))
	}
	if order.CustomerName != "" {
		b.WriteString(fmt.Sprintf("Customer: %s\n", order.CustomerName))
	}
	b.WriteString(time.Now().Format("02 Jan 2006 03:04 PM"))
	b.WriteByte('\n')
	b.WriteString(divider)
	b.WriteByte('\n')
	const rightBlock = 19
	nameWidth := width - rightBlock
	if nameWidth < 8 {
		nameWidth = 8
	}
	b.WriteString(fmt.Sprintf("%-*s%3s %7s %7s\n", nameWidth, "Item", "Qty", "Rate", "Price"))
	blocklist := ParseCategoryDisplayBlocklist(restaurant.CategoryDisplayBlocklist)
	for _, it := range items {
		name := printItemDisplayName(it, blocklist)
		lineTotal := it.Total
		if lineTotal <= 0 {
			lineTotal = it.UnitRate * float64(it.Quantity)
		}
		rate := it.UnitRate
		if rate <= 0 && it.Quantity > 0 {
			rate = lineTotal / float64(it.Quantity)
		}
		nameLines := wrapPrintWords(name, nameWidth)
		first := nameLines[0]
		if len(first) < nameWidth {
			first = first + strings.Repeat(" ", nameWidth-len(first))
		} else if len(first) > nameWidth {
			first = first[:nameWidth]
		}
		b.WriteString(fmt.Sprintf("%s%3d %7.2f %7.2f\n", first, it.Quantity, rate, lineTotal))
		for i := 1; i < len(nameLines); i++ {
			b.WriteString(nameLines[i])
			b.WriteByte('\n')
		}
	}
	b.WriteString(divider)
	b.WriteByte('\n')
	if order.SubTotal > 0 {
		b.WriteString(printPadLine("Subtotal", fmt.Sprintf("%.2f", order.SubTotal), width))
		b.WriteByte('\n')
	}
	if order.TaxAmount > 0 {
		b.WriteString(printPadLine("GST", fmt.Sprintf("%.2f", order.TaxAmount), width))
		b.WriteByte('\n')
	}
	if order.DiscountAmount > 0 {
		b.WriteString(printPadLine("Discount", fmt.Sprintf("-%.2f", order.DiscountAmount), width))
		b.WriteByte('\n')
	}
	b.WriteString(printPadLine("TOTAL", fmt.Sprintf("Rs.%.2f", order.Total), width))
	b.WriteByte('\n')
	if order.PaymentMethod != "" {
		b.WriteString(fmt.Sprintf("Payment: %s\n", strings.ToUpper(order.PaymentMethod)))
	}
	b.WriteString(divider)
	b.WriteByte('\n')
	b.WriteString(printCenterLine("Thank you!", width))
	b.WriteByte('\n')
	return b.String()
}

func wrapPrintWords(text string, width int) []string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return []string{""}
	}
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	current := ""
	for _, word := range fields {
		if current == "" {
			if len(word) <= width {
				current = word
			} else {
				for len(word) > width {
					lines = append(lines, word[:width])
					word = word[width:]
				}
				current = word
			}
			continue
		}
		if len(current)+1+len(word) <= width {
			current = current + " " + word
		} else {
			lines = append(lines, current)
			if len(word) <= width {
				current = word
			} else {
				for len(word) > width {
					lines = append(lines, word[:width])
					word = word[width:]
				}
				current = word
			}
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func printPadLine(left, right string, width int) string {
	if width <= 0 {
		width = 32
	}
	r := right
	if len(r) > width {
		r = r[len(r)-width:]
	}
	maxLeft := width - len(r) - 1
	if maxLeft < 0 {
		maxLeft = 0
	}
	l := left
	if len(l) > maxLeft {
		if maxLeft <= 1 {
			l = ""
		} else {
			l = l[:maxLeft-1] + "."
		}
	}
	spaces := width - len(l) - len(r)
	if spaces < 1 {
		spaces = 1
	}
	return l + strings.Repeat(" ", spaces) + r
}

func printCenterLine(text string, width int) string {
	t := strings.TrimSpace(text)
	if width <= 0 {
		width = 32
	}
	if len(t) > width {
		t = t[:width]
	}
	pad := width - len(t)
	if pad <= 0 {
		return t
	}
	return strings.Repeat(" ", pad/2) + t
}
