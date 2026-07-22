package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"restaurant-api/internal/middleware"
	"restaurant-api/internal/models"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ExpenseHandler struct {
	db           *gorm.DB
	orderService *services.OrderService
}

func NewExpenseHandler(db *gorm.DB) *ExpenseHandler {
	return &ExpenseHandler{
		db:           db,
		orderService: services.NewOrderService(db),
	}
}

type CreateExpenseRequest struct {
	Name   string  `json:"name" binding:"required"`
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

type ExpenseLine struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Amount    float64   `json:"amount"`
	Source    string    `json:"source"` // manual | stock
	CreatedAt time.Time `json:"created_at"`
}

func parseYearMonth(c *gin.Context) (int, int, time.Time, time.Time, error) {
	loc := services.RestaurantLocation()
	now := time.Now().In(loc)
	year := now.Year()
	month := int(now.Month())

	if y := strings.TrimSpace(c.Query("year")); y != "" {
		parsed, err := strconv.Atoi(y)
		if err != nil || parsed < 2000 || parsed > 2100 {
			return 0, 0, time.Time{}, time.Time{}, errInvalidYear
		}
		year = parsed
	}
	if m := strings.TrimSpace(c.Query("month")); m != "" {
		parsed, err := strconv.Atoi(m)
		if err != nil || parsed < 1 || parsed > 12 {
			return 0, 0, time.Time{}, time.Time{}, errInvalidMonth
		}
		month = parsed
	}

	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	return year, month, start, end, nil
}

var (
	errInvalidYear  = &apiError{msg: "invalid year"}
	errInvalidMonth = &apiError{msg: "invalid month"}
)

type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

func monthPeriodLabel(year, month int) string {
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("January 2006")
}

func sumStockExpenditure(db *gorm.DB, restaurantID string, start, end time.Time) (float64, error) {
	var total float64
	err := db.Model(&models.StockExpenditure{}).
		Where("restaurant_id = ? AND created_at >= ? AND created_at < ?", restaurantID, start.UTC(), end.UTC()).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

func sumManualExpenses(db *gorm.DB, restaurantID string, start, end time.Time) (float64, error) {
	var total float64
	err := db.Model(&models.Expense{}).
		Where("restaurant_id = ? AND created_at >= ? AND created_at < ?", restaurantID, start.UTC(), end.UTC()).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

// CreateExpense adds a named expense to the restaurant ledger.
func (h *ExpenseHandler) CreateExpense(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	var req CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}

	expense := models.Expense{
		RestaurantID: restaurantID.(string),
		Name:         name,
		Amount:       req.Amount,
		CreatedBy:    contextUserID(c),
	}
	if err := h.db.Create(&expense).Error; err != nil {
		log.Printf("❌ Create expense failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save expense"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Expense added",
		"expense": expense,
	})
}

// ListExpenses returns manual expenses for a month plus stock + total spend.
func (h *ExpenseHandler) ListExpenses(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	year, month, start, end, err := parseYearMonth(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var manual []models.Expense
	if err := h.db.Where("restaurant_id = ? AND created_at >= ? AND created_at < ?",
		restaurantID.(string), start.UTC(), end.UTC()).
		Order("created_at DESC").
		Find(&manual).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	manualTotal, err := sumManualExpenses(h.db, restaurantID.(string), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stockTotal, err := sumStockExpenditure(h.db, restaurantID.(string), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"year":             year,
		"month":            month,
		"period_label":     monthPeriodLabel(year, month),
		"expenses":         manual,
		"manual_total":     manualTotal,
		"stock_total":      stockTotal,
		"total":            manualTotal + stockTotal,
		"currency":         "INR",
		"period_start":     start.Format(time.RFC3339),
		"period_end":       end.Format(time.RFC3339),
	})
}

// DeleteExpense removes a manual expense entry.
func (h *ExpenseHandler) DeleteExpense(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	id := c.Param("expense_id")
	res := h.db.Where("id = ? AND restaurant_id = ?", id, restaurantID.(string)).Delete(&models.Expense{})
	if res.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error.Error()})
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "expense not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Expense deleted"})
}

// SettleReport builds a monthly settlement report: expenses + sales KPIs + top items.
func (h *ExpenseHandler) SettleReport(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "restaurant info not found"})
		return
	}

	year, month, start, end, err := parseYearMonth(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	manualTotal, err := sumManualExpenses(h.db, restaurantID.(string), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stockTotal, err := sumStockExpenditure(h.db, restaurantID.(string), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	totalExpenses := manualTotal + stockTotal

	revenue, orders, aov, topItems, err := h.orderService.SalesStatsForRange(
		restaurantID.(string), start.UTC(), end.UTC(), 5,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var restaurant models.Restaurant
	_ = h.db.Select("id", "name").Where("id = ?", restaurantID.(string)).First(&restaurant).Error

	var manual []models.Expense
	_ = h.db.Where("restaurant_id = ? AND created_at >= ? AND created_at < ?",
		restaurantID.(string), start.UTC(), end.UTC()).
		Order("created_at DESC").
		Find(&manual).Error

	var stockRows []models.StockExpenditure
	_ = h.db.Where("restaurant_id = ? AND created_at >= ? AND created_at < ?",
		restaurantID.(string), start.UTC(), end.UTC()).
		Order("created_at DESC").
		Find(&stockRows).Error

	lines := make([]ExpenseLine, 0, len(manual)+len(stockRows))
	for _, e := range manual {
		lines = append(lines, ExpenseLine{
			ID: e.ID, Name: e.Name, Amount: e.Amount, Source: "manual", CreatedAt: e.CreatedAt,
		})
	}
	for _, s := range stockRows {
		name := s.IngredientName
		if name == "" {
			name = "Stock refill"
		} else {
			name = "Stock refill: " + name
		}
		lines = append(lines, ExpenseLine{
			ID: s.ID, Name: name, Amount: s.Amount, Source: "stock", CreatedAt: s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"year":                year,
		"month":               month,
		"period_label":        monthPeriodLabel(year, month),
		"period_start":        start.Format(time.RFC3339),
		"period_end":          end.Format(time.RFC3339),
		"restaurant_name":     restaurant.Name,
		"currency":            "INR",
		"total_expenses":      totalExpenses,
		"manual_expenses":     manualTotal,
		"stock_expenses":      stockTotal,
		"total_orders":        orders,
		"total_revenue":       revenue,
		"average_order_value": aov,
		"net":                 revenue - totalExpenses,
		"top_items":           topItems,
		"expense_lines":       lines,
		"generated_at":        time.Now().UTC().Format(time.RFC3339),
	})
}

// SetupExpenseRoutes registers expense endpoints (admin/manager).
func SetupExpenseRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	handler := NewExpenseHandler(db)

	protected := router.Group("/expenses")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	protected.Use(middleware.RoleMiddleware("admin", "manager"))
	{
		protected.GET("", handler.ListExpenses)
		protected.POST("", handler.CreateExpense)
		protected.GET("/settle-report", handler.SettleReport)
		protected.DELETE("/:expense_id", handler.DeleteExpense)
	}

	log.Println("✅ Expense routes registered (admin/manager)")
}
