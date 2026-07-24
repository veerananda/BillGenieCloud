package handlers

import (
	"encoding/json"

	"restaurant-api/internal/models"

	"github.com/gin-gonic/gin"
)

func roleFromContext(c *gin.Context) string {
	if role, ok := c.Get("role"); ok {
		if s, ok := role.(string); ok {
			return s
		}
	}
	return ""
}

func canViewCostPrice(c *gin.Context) bool {
	return roleFromContext(c) == "admin"
}

func orderMenuItemPayload(item *models.MenuItem, includeCost bool) map[string]interface{} {
	if item == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":            item.ID,
		"name":          item.Name,
		"description":   item.Description,
		"price":         item.Price,
		"is_veg":        item.IsVeg,
		"is_vegetarian": item.IsVeg,
		"is_available":  item.IsAvailable,
		"category":      item.Category,
		"restaurant_id": item.RestaurantID,
	}
	if includeCost {
		m["cost_price"] = item.CostPrice
	}
	return m
}

// menuItemsForResponse strips cost_price for non-admin callers.
func menuItemsForResponse(items []models.MenuItem, includeCost bool) interface{} {
	if includeCost {
		return items
	}
	out := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		b, err := json.Marshal(items[i])
		if err != nil {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		delete(m, "cost_price")
		out = append(out, m)
	}
	return out
}

func menuItemForResponse(item models.MenuItem, includeCost bool) interface{} {
	if includeCost {
		return item
	}
	b, err := json.Marshal(item)
	if err != nil {
		return item
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return item
	}
	delete(m, "cost_price")
	return m
}
