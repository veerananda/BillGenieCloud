package handlers

import (
	"log"
	"net/http"
	"restaurant-api/internal/models"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TableHandler struct {
	db *gorm.DB
}

func NewTableHandler(db *gorm.DB) *TableHandler {
	return &TableHandler{db: db}
}

// GetTables retrieves all tables for a restaurant
// GET /tables
func (h *TableHandler) GetTables(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var tables []models.RestaurantTable
	if err := h.db.Where("restaurant_id = ?", restaurantID).
		Order("created_at ASC").
		Find(&tables).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tables"})
		return
	}

	c.JSON(http.StatusOK, tables)
}

// CreateTable creates a new table
// POST /tables
func (h *TableHandler) CreateTable(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if table already exists
	var existing models.RestaurantTable
	if err := h.db.Where("restaurant_id = ? AND name = ?", restaurantID, req.Name).
		First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Table already exists"})
		return
	}

	table := models.RestaurantTable{
		RestaurantID: restaurantID,
		Name:         req.Name,
		IsOccupied:   false,
	}

	if err := h.db.Create(&table).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create table"})
		return
	}

	c.JSON(http.StatusCreated, table)
}

// CreateBulkTables creates multiple tables at once
// POST /tables/bulk
func (h *TableHandler) CreateBulkTables(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Names string `json:"names" binding:"required"` // Comma or newline separated
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse names - support both comma and newline separated
	var names []string
	if strings.Contains(req.Names, ",") {
		names = strings.Split(req.Names, ",")
	} else {
		names = strings.Split(req.Names, "\n")
	}

	var tables []models.RestaurantTable
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Check if table already exists
		var existing models.RestaurantTable
		if err := h.db.Where("restaurant_id = ? AND name = ?", restaurantID, name).
			First(&existing).Error; err == nil {
			// Table already exists, skip
			continue
		}

		table := models.RestaurantTable{
			RestaurantID: restaurantID,
			Name:         name,
			IsOccupied:   false,
		}

		if err := h.db.Create(&table).Error; err == nil {
			tables = append(tables, table)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Tables created successfully",
		"count":   len(tables),
		"tables":  tables,
	})
}

// UpdateTable updates a table
// PUT /tables/:id
func (h *TableHandler) UpdateTable(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tableID := c.Param("id")

	var req struct {
		Name       string `json:"name"`
		IsOccupied *bool  `json:"is_occupied"`
		Capacity   *int   `json:"capacity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var table models.RestaurantTable
	if err := h.db.Where("id = ? AND restaurant_id = ?", tableID, restaurantID).
		First(&table).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
		return
	}

	// Update fields if provided
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.IsOccupied != nil {
		updates["is_occupied"] = *req.IsOccupied
	}
	if req.Capacity != nil {
		updates["capacity"] = *req.Capacity
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	if err := h.db.Model(&table).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update table"})
		return
	}

	c.JSON(http.StatusOK, table)
}

// DeleteTable deletes a table
// DELETE /tables/:id
func (h *TableHandler) DeleteTable(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tableID := c.Param("id")

	var table models.RestaurantTable
	if err := h.db.Where("id = ? AND restaurant_id = ?", tableID, restaurantID).
		First(&table).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
		return
	}

	if err := h.db.Delete(&table).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete table"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Table deleted successfully"})
}

// SetTableOccupied sets table as occupied with an order
// PUT /tables/:id/occupy
func (h *TableHandler) SetTableOccupied(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tableID := c.Param("id")
	log.Printf("📌 SetTableOccupied called: tableID=%s, restaurantID=%s", tableID, restaurantID)

	var req struct {
		OrderID string `json:"order_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ SetTableOccupied binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("📌 SetTableOccupied request: orderID=%s", req.OrderID)

	var table models.RestaurantTable
	if err := h.db.Where("id = ? AND restaurant_id = ?", tableID, restaurantID).
		First(&table).Error; err != nil {
		log.Printf("❌ SetTableOccupied table not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
		return
	}

	if err := h.db.Model(&table).Updates(map[string]interface{}{
		"is_occupied":      true,
		"current_order_id": req.OrderID,
	}).Error; err != nil {
		log.Printf("❌ SetTableOccupied update error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update table"})
		return
	}

	log.Printf("✅ SetTableOccupied success: Table %s now occupied with order %s", tableID, req.OrderID)
	c.JSON(http.StatusOK, table)
}

// SetTableVacant sets table as vacant
// PUT /tables/:id/vacant
func (h *TableHandler) SetTableVacant(c *gin.Context) {
	restaurantID := c.GetString("restaurant_id")
	if restaurantID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	tableID := c.Param("id")

	var table models.RestaurantTable
	if err := h.db.Where("id = ? AND restaurant_id = ?", tableID, restaurantID).
		First(&table).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Table not found"})
		return
	}

	if err := h.db.Model(&table).Updates(map[string]interface{}{
		"is_occupied":      false,
		"current_order_id": nil,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update table"})
		return
	}

	c.JSON(http.StatusOK, table)
}
