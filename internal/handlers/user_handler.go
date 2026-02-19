package handlers

import (
	"log"
	"net/http"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type UserHandler struct {
	userService *services.UserService
	validator   *validator.Validate
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
		validator:   validator.New(),
	}
}

// ListUsers retrieves all staff members for a restaurant (Admin only)
// @Summary List staff users
// @Description Get all staff and manager users for a restaurant
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param role query string false "Filter by role: manager or staff"
// @Param is_active query boolean false "Filter by active status"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Router /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	// Get restaurant ID from context (set by middleware)
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	// Build filters
	filters := make(map[string]interface{})

	if role := c.Query("role"); role != "" {
		filters["role"] = role
	}

	if isActive := c.Query("is_active"); isActive == "true" {
		filters["is_active"] = true
	} else if isActive == "false" {
		filters["is_active"] = false
	}

	// Fetch users
	users, err := h.userService.ListUsers(restaurantID.(string), filters)
	if err != nil {
		log.Printf("❌ Error listing users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get staff count for context
	staffCount, _ := h.userService.GetRestaurantStaffCount(restaurantID.(string))

	c.JSON(http.StatusOK, gin.H{
		"message":       "Staff list retrieved successfully",
		"restaurant_id": restaurantID,
		"staff":         users,
		"total_count":   len(users),
		"staff_count":   staffCount,
	})

	log.Printf("✅ Listed %d staff members for restaurant %s", len(users), restaurantID)
}

// CreateUser creates a new staff or manager user (Admin only)
// @Summary Create staff user
// @Description Create a new staff or manager user for the restaurant
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body services.CreateUserRequest true "User creation data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Router /users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req services.CreateUserRequest

	// Get restaurant ID from context
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	// Bind request body
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		log.Printf("❌ Validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create user
	user, err := h.userService.CreateUser(restaurantID.(string), req)
	if err != nil {
		log.Printf("❌ Error creating user: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Staff user created successfully",
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"restaurant_id": user.RestaurantID,
			"is_active":     user.IsActive,
			"created_at":    user.CreatedAt,
		},
	})

	log.Printf("✅ Staff user created: %s (Role: %s)", user.Email, user.Role)
}

// GetUser retrieves details of a specific user (Admin only)
// @Summary Get user details
// @Description Get detailed information about a specific staff member
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Router /users/{user_id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("user_id")
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	user, err := h.userService.GetUser(userID)
	if err != nil {
		log.Printf("❌ Error getting user: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Verify user belongs to the restaurant
	if user.RestaurantID != restaurantID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "user does not belong to this restaurant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"restaurant_id": user.RestaurantID,
			"is_active":     user.IsActive,
			"created_at":    user.CreatedAt,
			"updated_at":    user.UpdatedAt,
		},
	})

	log.Printf("✅ Retrieved user: %s", userID)
}

// UpdateUser updates a staff user's information (Admin only)
// @Summary Update staff user
// @Description Update staff user name, phone, role, or active status
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Param request body services.UpdateUserRequest true "Update data"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Router /users/{user_id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("user_id")
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	var req services.UpdateUserRequest

	// Bind request body
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request (if fields are provided)
	if err := h.validator.Struct(req); err != nil {
		log.Printf("❌ Validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update user
	user, err := h.userService.UpdateUser(userID, restaurantID.(string), req)
	if err != nil {
		log.Printf("❌ Error updating user: %v", err)

		// Return 404 if user not found or doesn't belong to restaurant
		if err.Error() == "user not found" || err.Error() == "user does not belong to this restaurant" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff user updated successfully",
		"user": gin.H{
			"id":            user.ID,
			"name":          user.Name,
			"email":         user.Email,
			"phone":         user.Phone,
			"role":          user.Role,
			"restaurant_id": user.RestaurantID,
			"is_active":     user.IsActive,
			"updated_at":    user.UpdatedAt,
		},
	})

	log.Printf("✅ Staff user updated: %s", userID)
}

// DeleteUser deletes (soft-delete) a staff user (Admin only)
// @Summary Delete staff user
// @Description Soft-delete a staff member (sets is_active to false)
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /users/{user_id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("user_id")
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	// Delete user
	err := h.userService.DeleteUser(userID, restaurantID.(string))
	if err != nil {
		log.Printf("❌ Error deleting user: %v", err)

		// Return appropriate status code based on error
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if err.Error() == "user does not belong to this restaurant" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff user deleted successfully",
		"user_id": userID,
	})

	log.Printf("✅ Staff user deleted: %s", userID)
}

// RestoreUser reactivates a deleted (inactive) staff user (Admin only)
// @Summary Restore staff user
// @Description Reactivate a soft-deleted staff member
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Router /users/{user_id}/restore [post]
func (h *UserHandler) RestoreUser(c *gin.Context) {
	userID := c.Param("user_id")
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	// Restore user
	err := h.userService.RestoreUser(userID, restaurantID.(string))
	if err != nil {
		log.Printf("❌ Error restoring user: %v", err)

		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if err.Error() == "user does not belong to this restaurant" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Staff user restored successfully",
		"user_id": userID,
	})

	log.Printf("✅ Staff user restored: %s", userID)
}

// GetStaffStats retrieves statistics about staff members (Admin only)
// @Summary Get staff statistics
// @Description Get statistics about staff members in the restaurant
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /users/stats [get]
func (h *UserHandler) GetStaffStats(c *gin.Context) {
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	staffCount, _ := h.userService.GetRestaurantStaffCount(restaurantID.(string))
	adminCount, _ := h.userService.GetAdminCount(restaurantID.(string))

	c.JSON(http.StatusOK, gin.H{
		"message":       "Staff statistics",
		"restaurant_id": restaurantID,
		"total_staff":   staffCount,
		"total_admins":  adminCount,
		"timestamp":     gin.H{},
	})

	log.Printf("✅ Retrieved staff stats for restaurant %s", restaurantID)
}

// RegenerateStaffKey regenerates the staff key for a user
// @Summary Regenerate staff key
// @Description Regenerates a new unique staff key and optionally password for a staff member
// @Tags users
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Param RegenerateStaffKeyRequest body services.RegenerateStaffKeyRequest false "Optional new password"
// @Success 200 {object} map[string]interface{} "New staff key generated"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "User not found"
// @Router /users/{user_id}/regenerate-key [post]
func (h *UserHandler) RegenerateStaffKey(c *gin.Context) {
	userID := c.Param("user_id")
	restaurantID, exists := c.Get("restaurant_id")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "restaurant_id not found in context"})
		return
	}

	// Check if the user exists and belongs to this restaurant
	user, err := h.userService.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Verify user belongs to this restaurant
	if user.RestaurantID != restaurantID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only manage staff from your restaurant"})
		return
	}

	// Parse optional request body (new password)
	var req services.RegenerateStaffKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// It's okay if no body provided (password is optional)
		req = services.RegenerateStaffKeyRequest{}
	}

	// Regenerate the staff key (and password if provided)
	newKey, err := h.userService.RegenerateStaffKey(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to regenerate staff key"})
		return
	}

	responseMsg := "Staff key regenerated successfully"
	if req.NewPassword != nil && *req.NewPassword != "" {
		responseMsg += " (password also updated)"
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   responseMsg,
		"staff_key": newKey,
		"user_id":   userID,
	})

	log.Printf("✅ Regenerated staff key for user %s in restaurant %s", userID, restaurantID)
}
