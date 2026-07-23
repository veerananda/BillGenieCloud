package services

import (
	"errors"
	"strings"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VariantInput is used when creating/updating menu item portion options.
type VariantInput struct {
	ID          string  `json:"id,omitempty"`
	Label       string  `json:"label"`
	Price       float64 `json:"price"`
	RecipeScale float64 `json:"recipe_scale"`
	IsDefault   bool    `json:"is_default"`
	IsAvailable *bool   `json:"is_available,omitempty"`
	SortOrder   int     `json:"sort_order"`
}

func normalizeRecipeScale(scale float64) float64 {
	if scale <= 0 {
		return 1
	}
	return scale
}

// EnsureDefaultMenuItemVariant creates a Regular variant when none exist.
func EnsureDefaultMenuItemVariant(db *gorm.DB, item models.MenuItem) error {
	var count int64
	if err := db.Model(&models.MenuItemVariant{}).
		Where("menu_item_id = ? AND restaurant_id = ?", item.ID, item.RestaurantID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	v := models.MenuItemVariant{
		ID:           uuid.New().String(),
		RestaurantID: item.RestaurantID,
		MenuItemID:   item.ID,
		Label:        "Regular",
		Price:        item.Price,
		RecipeScale:  1,
		IsDefault:    true,
		IsAvailable:  true,
		SortOrder:    0,
	}
	return db.Create(&v).Error
}

// SyncMenuItemVariants replaces variants for a menu item from the provided list.
// If variants is empty, ensures a single Regular variant matching item.Price.
func SyncMenuItemVariants(db *gorm.DB, item models.MenuItem, variants []VariantInput) ([]models.MenuItemVariant, error) {
	if len(variants) == 0 {
		if err := EnsureDefaultMenuItemVariant(db, item); err != nil {
			return nil, err
		}
		var out []models.MenuItemVariant
		if err := db.Where("menu_item_id = ?", item.ID).Order("sort_order ASC, created_at ASC").Find(&out).Error; err != nil {
			return nil, err
		}
		return out, nil
	}

	keepIDs := make([]string, 0, len(variants))
	hasDefault := false
	created := make([]models.MenuItemVariant, 0, len(variants))

	for i, in := range variants {
		label := strings.TrimSpace(in.Label)
		if label == "" {
			return nil, errors.New("variant label is required")
		}
		if in.Price < 0 {
			return nil, errors.New("variant price cannot be negative")
		}
		available := true
		if in.IsAvailable != nil {
			available = *in.IsAvailable
		}
		scale := normalizeRecipeScale(in.RecipeScale)
		isDefault := in.IsDefault
		if isDefault {
			hasDefault = true
		}
		sortOrder := in.SortOrder
		if sortOrder == 0 {
			sortOrder = i
		}

		if strings.TrimSpace(in.ID) != "" {
			var existing models.MenuItemVariant
			if err := db.Where("id = ? AND menu_item_id = ? AND restaurant_id = ?", in.ID, item.ID, item.RestaurantID).
				First(&existing).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil, errors.New("variant not found for menu item")
				}
				return nil, err
			}
			existing.Label = label
			existing.Price = in.Price
			existing.RecipeScale = scale
			existing.IsDefault = isDefault
			existing.IsAvailable = available
			existing.SortOrder = sortOrder
			if err := db.Save(&existing).Error; err != nil {
				return nil, err
			}
			keepIDs = append(keepIDs, existing.ID)
			created = append(created, existing)
			continue
		}

		row := models.MenuItemVariant{
			ID:           uuid.New().String(),
			RestaurantID: item.RestaurantID,
			MenuItemID:   item.ID,
			Label:        label,
			Price:        in.Price,
			RecipeScale:  scale,
			IsDefault:    isDefault,
			IsAvailable:  available,
			SortOrder:    sortOrder,
		}
		if err := db.Create(&row).Error; err != nil {
			return nil, err
		}
		keepIDs = append(keepIDs, row.ID)
		created = append(created, row)
	}

	if !hasDefault && len(created) > 0 {
		created[0].IsDefault = true
		if err := db.Model(&models.MenuItemVariant{}).
			Where("id = ?", created[0].ID).
			Update("is_default", true).Error; err != nil {
			return nil, err
		}
	}

	// Clear default flag on non-default rows
	defaultID := ""
	for _, v := range created {
		if v.IsDefault {
			defaultID = v.ID
			break
		}
	}
	if defaultID != "" {
		_ = db.Model(&models.MenuItemVariant{}).
			Where("menu_item_id = ? AND id <> ?", item.ID, defaultID).
			Update("is_default", false).Error
	}

	if len(keepIDs) > 0 {
		if err := db.Where("menu_item_id = ? AND id NOT IN ?", item.ID, keepIDs).
			Delete(&models.MenuItemVariant{}).Error; err != nil {
			return nil, err
		}
	}

	// Keep parent price aligned with default variant for backward compatibility.
	for _, v := range created {
		if v.IsDefault {
			_ = db.Model(&models.MenuItem{}).Where("id = ?", item.ID).Update("price", v.Price).Error
			break
		}
	}

	var out []models.MenuItemVariant
	if err := db.Where("menu_item_id = ?", item.ID).Order("sort_order ASC, created_at ASC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// ResolveOrderVariant picks the variant for an order line.
// Empty variantID uses the default (or sole) variant; missing variants fall back to menu price.
func ResolveOrderVariant(db *gorm.DB, restaurantID, menuItemID, variantID string, menuPrice float64) (price float64, label string, scale float64, variantIDOut *string, err error) {
	price = menuPrice
	label = ""
	scale = 1

	var variants []models.MenuItemVariant
	if err := db.Where("menu_item_id = ? AND restaurant_id = ?", menuItemID, restaurantID).
		Order("sort_order ASC, created_at ASC").
		Find(&variants).Error; err != nil {
		return price, label, scale, nil, err
	}
	if len(variants) == 0 {
		return price, label, scale, nil, nil
	}

	var chosen *models.MenuItemVariant
	if strings.TrimSpace(variantID) != "" {
		for i := range variants {
			if variants[i].ID == variantID {
				chosen = &variants[i]
				break
			}
		}
		if chosen == nil {
			return 0, "", 0, nil, errors.New("variant does not belong to this menu item")
		}
		if !chosen.IsAvailable {
			return 0, "", 0, nil, errors.New("selected variant is not available")
		}
	} else {
		for i := range variants {
			if variants[i].IsDefault && variants[i].IsAvailable {
				chosen = &variants[i]
				break
			}
		}
		if chosen == nil {
			for i := range variants {
				if variants[i].IsAvailable {
					chosen = &variants[i]
					break
				}
			}
		}
		if chosen == nil {
			chosen = &variants[0]
		}
	}

	id := chosen.ID
	return chosen.Price, chosen.Label, normalizeRecipeScale(chosen.RecipeScale), &id, nil
}

// FormatOrderItemDisplayName returns "Dish (Half)" when variant is non-Regular.
func FormatOrderItemDisplayName(menuName, variantLabel string) string {
	name := strings.TrimSpace(menuName)
	if name == "" {
		name = "Item"
	}
	label := strings.TrimSpace(variantLabel)
	if label == "" || strings.EqualFold(label, "Regular") {
		return name
	}
	return name + " (" + label + ")"
}

// LoadVariantScalesByID returns recipe_scale for each variant id.
func LoadVariantScalesByID(db *gorm.DB, restaurantID string, variantIDs []string) (map[string]float64, error) {
	out := make(map[string]float64)
	if len(variantIDs) == 0 {
		return out, nil
	}
	var rows []models.MenuItemVariant
	if err := db.Where("restaurant_id = ? AND id IN ?", restaurantID, variantIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = normalizeRecipeScale(row.RecipeScale)
	}
	return out, nil
}
