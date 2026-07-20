package services

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var numericStaffKeyPattern = regexp.MustCompile(`^\d{6}$`)

type UserService struct {
	db *gorm.DB
}

// CreateUserRequest for creating new staff/manager/chef
// Note: Email is optional for staff/manager/chef (they use staff_key + password)
// Email is only required for admin during registration
// StaffKey is a 6-digit login key (generated on frontend or by the server).
type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2"`
	Email    string `json:"email" validate:"omitempty,email"`
	Phone    string `json:"phone"`
	Password string `json:"password" validate:"omitempty,min=6"`
	Role            string `json:"role" validate:"required,oneof=manager staff chef"`
	StaffKey        string `json:"staff_key"`
	CanCancelOrders      bool   `json:"can_cancel_orders"`
	CanRestockInventory  bool   `json:"can_restock_inventory"`
	MenuManagementAccess bool   `json:"menu_management_access"`
}

// UpdateUserRequest for updating existing staff/manager/chef
type UpdateUserRequest struct {
	Name            string `json:"name" validate:"omitempty,min=2"`
	Phone           string `json:"phone"`
	Password        string `json:"password" validate:"omitempty,min=6"` // Optional: only update if provided
	Role            string `json:"role" validate:"omitempty,oneof=manager staff chef"`
	IsActive        *bool  `json:"is_active"`
	CanCancelOrders      *bool  `json:"can_cancel_orders"`
	CanRestockInventory  *bool  `json:"can_restock_inventory"`
	MenuManagementAccess *bool  `json:"menu_management_access"`
}

// NewUserService creates a new user service
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		db: db,
	}
}

// CreateUser creates a new staff, manager, or chef user for a restaurant
func (s *UserService) CreateUser(restaurantID string, req CreateUserRequest) (*models.User, error) {
	// Validate role
	if req.Role != "manager" && req.Role != "staff" && req.Role != "chef" {
		return nil, errors.New("invalid role. must be 'manager', 'staff', or 'chef'")
	}

	// Enforce subscription limits for the requested role.
	var userCounts struct {
		AdminCount   int64
		ManagerCount int64
		StaffCount   int64
		ChefCount    int64
	}

	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "admin", true).Count(&userCounts.AdminCount)
	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "manager", true).Count(&userCounts.ManagerCount)
	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "staff", true).Count(&userCounts.StaffCount)
	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "chef", true).Count(&userCounts.ChefCount)

	log.Printf("📊 Current user counts for restaurant %s: Admin=%d, Manager=%d, Staff=%d, Chef=%d",
		restaurantID, userCounts.AdminCount, userCounts.ManagerCount, userCounts.StaffCount, userCounts.ChefCount)

	// Enforce limits
	if err := EnforceCreateUser(s.db, restaurantID, req.Role); err != nil {
		return nil, err
	}

	staffKey := strings.TrimSpace(req.StaffKey)
	if staffKey == "" {
		generated, genErr := generateUniqueNumericStaffKey(s.db)
		if genErr != nil {
			return nil, genErr
		}
		staffKey = generated
	} else if !numericStaffKeyPattern.MatchString(staffKey) {
		return nil, errors.New("staff key must be a 6-digit number")
	}

	// For staff/manager/chef, if email not provided, use the staff_key as email (it's globally unique)
	if req.Email == "" {
		req.Email = staffKey
	}

	// Check if email already exists in this restaurant (only if email is provided)
	if req.Email != "" {
		var existingUser models.User
		if err := s.db.Where("restaurant_id = ? AND email = ?", restaurantID, req.Email).First(&existingUser).Error; err == nil {
			return nil, errors.New("email already exists in this restaurant")
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("database error: %w", err)
		}
	}

	var existingKeyUser models.User
	if err := s.db.Where("staff_key = ?", staffKey).First(&existingKeyUser).Error; err == nil {
		return nil, errors.New("staff key already in use, please regenerate a new key")
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("database error: %w", err)
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = staffKey
	}

	// Hash password
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("password hashing failed: %w", err)
	}

	// Create new user
	canCancelOrders := false
	if req.Role == "staff" && req.CanCancelOrders {
		canCancelOrders = true
	}

	canRestockInventory := false
	if (req.Role == "staff" || req.Role == "chef") && req.CanRestockInventory {
		canRestockInventory = true
	}

	menuManagementAccess := false
	if req.Role == "manager" && req.MenuManagementAccess {
		menuManagementAccess = true
	}

	user := &models.User{
		ID:                  uuid.New().String(),
		RestaurantID:        restaurantID,
		Name:                req.Name,
		Email:               req.Email,
		Phone:               req.Phone,
		PasswordHash:        hashedPassword,
		Role:                req.Role,
		StaffKey:            staffKey,
		IsActive:            true,
		CanCancelOrders:      canCancelOrders,
		CanRestockInventory:  canRestockInventory,
		MenuManagementAccess: menuManagementAccess,
	}

	// Save to database
	if err := s.db.Create(user).Error; err != nil {
		log.Printf("❌ Failed to create user: %v", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("✅ User created: %s (StaffKey: %s, ID: %s, Role: %s, Restaurant: %s)", user.Name, user.StaffKey, user.ID, user.Role, restaurantID)
	return user, nil
}

// ListUsers retrieves all staff users for a restaurant
func (s *UserService) ListUsers(restaurantID string, filters map[string]interface{}) ([]models.User, error) {
	var users []models.User

	query := s.db.Where("restaurant_id = ? AND role IN ('manager', 'staff', 'chef', 'admin')", restaurantID)

	// Default: show only active users (unless explicitly requesting inactive)
	if isActive, ok := filters["is_active"].(bool); ok {
		query = query.Where("is_active = ?", isActive)
	} else {
		// By default, show only active users (hide deleted ones)
		query = query.Where("is_active = ?", true)
	}

	// Apply role filter if provided
	if role, ok := filters["role"].(string); ok && role != "" {
		query = query.Where("role = ?", role)
	}

	// Sort by creation date descending
	query = query.Order("created_at DESC")

	if err := query.Find(&users).Error; err != nil {
		log.Printf("❌ Failed to list users: %v", err)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	log.Printf("📋 Retrieved %d users for restaurant %s", len(users), restaurantID)
	return users, nil
}

// GetUser retrieves a specific user by ID
func (s *UserService) GetUser(userID string) (*models.User, error) {
	var user models.User

	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email within a restaurant
func (s *UserService) GetUserByEmail(restaurantID, email string) (*models.User, error) {
	var user models.User

	if err := s.db.Where("restaurant_id = ? AND email = ?", restaurantID, email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("database error: %w", err)
	}

	return &user, nil
}

// UpdateUser updates user information
func (s *UserService) UpdateUser(userID string, restaurantID string, req UpdateUserRequest) (*models.User, error) {
	// Fetch existing user
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	// Verify user belongs to the restaurant
	if user.RestaurantID != restaurantID {
		return nil, errors.New("user does not belong to this restaurant")
	}

	isAdmin := user.Role == "admin"

	// Update fields
	updates := map[string]interface{}{}

	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.Phone != "" || isAdmin {
		updates["phone"] = req.Phone
	}

	if !isAdmin && req.Role != "" {
		if req.Role != "manager" && req.Role != "staff" && req.Role != "chef" {
			return nil, errors.New("invalid role")
		}
		if req.Role != user.Role {
			if err := EnforceCreateUser(s.db, restaurantID, req.Role); err != nil {
				return nil, err
			}
			updates["role"] = req.Role
			if req.Role != "staff" {
				updates["can_cancel_orders"] = false
			}
			if req.Role == "manager" {
				updates["can_restock_inventory"] = false
			}
			if req.Role != "manager" {
				updates["menu_management_access"] = false
			}
		}
	}

	if req.CanCancelOrders != nil {
		effectiveRole := user.Role
		if !isAdmin && req.Role != "" {
			effectiveRole = req.Role
		}
		if effectiveRole == "staff" {
			updates["can_cancel_orders"] = *req.CanCancelOrders
		}
	}

	if req.CanRestockInventory != nil {
		effectiveRole := user.Role
		if !isAdmin && req.Role != "" {
			effectiveRole = req.Role
		}
		if effectiveRole == "staff" || effectiveRole == "chef" {
			updates["can_restock_inventory"] = *req.CanRestockInventory
		}
	}

	if req.MenuManagementAccess != nil {
		effectiveRole := user.Role
		if !isAdmin && req.Role != "" {
			effectiveRole = req.Role
		}
		if effectiveRole == "manager" {
			updates["menu_management_access"] = *req.MenuManagementAccess
		}
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	// Password is optional - only hash and update if provided
	if req.Password != "" {
		hashedPassword, err := hashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("password hashing failed: %w", err)
		}
		updates["password_hash"] = hashedPassword
	}

	// If no updates provided, return error
	if len(updates) == 0 {
		return nil, errors.New("no fields to update")
	}

	// Save updates
	if err := s.db.Model(user).Updates(updates).Error; err != nil {
		log.Printf("❌ Failed to update user: %v", err)
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Refresh user data
	if err := s.db.First(user).Error; err != nil {
		return nil, fmt.Errorf("failed to refresh user data: %w", err)
	}

	logMsg := "✅ User updated: " + user.Email + " (ID: " + user.ID
	if req.Password != "" {
		logMsg += ", password updated"
	}
	logMsg += ")"
	log.Printf("%s", logMsg)
	return user, nil
}

// UserCanCancelOrders reports whether a user may cancel dine-in table orders.
func UserCanCancelOrders(user *models.User) bool {
	if user == nil {
		return false
	}
	switch user.Role {
	case "admin", "manager":
		return true
	case "staff":
		return user.CanCancelOrders
	default:
		return false
	}
}

// UserCanAdjustOrderItems reports whether a user may remove or reduce line items on an active order.
// Owner (admin) and manager only — not staff even with can_cancel_orders.
func UserCanAdjustOrderItems(user *models.User) bool {
	if user == nil {
		return false
	}
	return user.Role == "admin" || user.Role == "manager"
}

// UserCanRestockInventory reports whether a user may add stock refills.
func UserCanRestockInventory(user *models.User) bool {
	if user == nil {
		return false
	}
	switch user.Role {
	case "admin", "manager":
		return true
	case "staff", "chef":
		return user.CanRestockInventory
	default:
		return false
	}
}

// UserCanManageMenu reports whether a user may add/edit/delete menu items and categories.
func UserCanManageMenu(user *models.User) bool {
	if user == nil {
		return false
	}
	switch user.Role {
	case "admin":
		return true
	case "manager":
		return user.MenuManagementAccess
	default:
		return false
	}
}

// UserCanToggleMenuAvailability reports whether a user may enable/disable item availability.
func UserCanToggleMenuAvailability(user *models.User) bool {
	if user == nil {
		return false
	}
	switch user.Role {
	case "admin", "manager":
		return true
	default:
		return false
	}
}

// DeleteUser soft-deletes a user by setting is_active to false
func (s *UserService) DeleteUser(userID string, restaurantID string) error {
	// Fetch user
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	// Verify user belongs to the restaurant
	if user.RestaurantID != restaurantID {
		return errors.New("user does not belong to this restaurant")
	}

	if user.Role == "admin" {
		return errors.New("admin account cannot be deleted")
	}

	// Hard delete with cleanup: Begin transaction
	tx := s.db.Begin()

	// 1. Set CreatedByUserID to NULL in orders table (keep order history intact)
	if err := tx.Model(&models.Order{}).Where("created_by_user_id = ?", userID).Update("created_by_user_id", nil).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to update orders: %v", err)
		return fmt.Errorf("failed to update orders: %w", err)
	}

	// 2. Set UserID to NULL in audit_logs table (keep audit trail)
	if err := tx.Model(&models.AuditLog{}).Where("user_id = ?", userID).Update("user_id", nil).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to update audit logs: %v", err)
		return fmt.Errorf("failed to update audit logs: %w", err)
	}

	// 3. Delete refresh tokens
	if err := tx.Where("user_id = ?", userID).Delete(&models.RefreshToken{}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to delete refresh tokens: %v", err)
		return fmt.Errorf("failed to delete refresh tokens: %w", err)
	}

	// 4. Delete user sessions
	if err := tx.Where("user_id = ?", userID).Delete(&models.UserSession{}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to delete user sessions: %v", err)
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	// 5. Delete password reset records
	if err := tx.Where("user_id = ?", userID).Delete(&models.PasswordReset{}).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to delete password reset records: %v", err)
		return fmt.Errorf("failed to delete password reset records: %w", err)
	}

	// 6. Hard delete the user record
	if err := tx.Delete(user).Error; err != nil {
		tx.Rollback()
		log.Printf("❌ Failed to hard delete user: %v", err)
		return fmt.Errorf("failed to hard delete user: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("❌ Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✅ User permanently deleted: %s (ID: %s)", user.Email, userID)
	return nil
}

// RestoreUser reactivates a soft-deleted user
func (s *UserService) RestoreUser(userID string, restaurantID string) error {
	// Fetch user
	user, err := s.GetUser(userID)
	if err != nil {
		return err
	}

	// Verify user belongs to the restaurant
	if user.RestaurantID != restaurantID {
		return errors.New("user does not belong to this restaurant")
	}

	// Reactivate
	if err := s.db.Model(user).Update("is_active", true).Update("updated_at", time.Now()).Error; err != nil {
		log.Printf("❌ Failed to restore user: %v", err)
		return fmt.Errorf("failed to restore user: %w", err)
	}

	log.Printf("✅ User restored: %s (ID: %s)", user.Email, userID)
	return nil
}

// GetRestaurantStaffCount returns the number of active staff members
func (s *UserService) GetRestaurantStaffCount(restaurantID string) (int64, error) {
	var count int64

	if err := s.db.Where("restaurant_id = ? AND role IN ('manager', 'staff') AND is_active = ?", restaurantID, true).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count staff: %w", err)
	}

	return count, nil
}

// GetAdminCount returns the number of active admin users for a restaurant
func (s *UserService) GetAdminCount(restaurantID string) (int64, error) {
	var count int64

	if err := s.db.Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "admin", true).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count admins: %w", err)
	}

	return count, nil
}

// RegenerateStaffKeyRequest for regenerating staff key and optionally password
type RegenerateStaffKeyRequest struct {
	NewPassword *string `json:"new_password" validate:"omitempty,min=6"` // Optional: if provided, password is also updated
}

// RegenerateStaffKey regenerates a new staff key and optionally updates password
func (s *UserService) RegenerateStaffKey(userID string, req RegenerateStaffKeyRequest) (string, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return "", err
	}

	if user.Role != "manager" && user.Role != "staff" && user.Role != "chef" {
		return "", errors.New("staff key can only be regenerated for manager/staff/chef users")
	}

	newStaffKey, err := generateUniqueNumericStaffKey(s.db)
	if err != nil {
		return "", err
	}

	// Prepare update map
	updateMap := map[string]interface{}{
		"staff_key":        newStaffKey,
		"key_generated_at": time.Now(),
		"updated_at":       time.Now(),
	}

	// Default password to the new key; override when admin sets a new password.
	passwordToSet := newStaffKey
	if req.NewPassword != nil && *req.NewPassword != "" {
		passwordToSet = *req.NewPassword
	}
	hashedPassword, err := hashPassword(passwordToSet)
	if err != nil {
		return "", fmt.Errorf("password hashing failed: %w", err)
	}
	updateMap["password_hash"] = hashedPassword

	if err := s.db.Model(user).Updates(updateMap).Error; err != nil {
		return "", fmt.Errorf("failed to regenerate staff key: %w", err)
	}

	logMsg := fmt.Sprintf("✅ Staff key regenerated for user: %s", userID)
	if req.NewPassword != nil && *req.NewPassword != "" {
		logMsg += " (password also updated)"
	}
	log.Printf("%s", logMsg)
	return newStaffKey, nil
}

func generateUniqueNumericStaffKey(db *gorm.DB) (string, error) {
	for i := 0; i < 30; i++ {
		candidate := fmt.Sprintf("%06d", 100000+rand.Intn(900000))
		var count int64
		if err := db.Model(&models.User{}).Where("staff_key = ?", candidate).Count(&count).Error; err != nil {
			return "", fmt.Errorf("failed to validate staff key uniqueness: %w", err)
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errors.New("failed to generate unique staff key")
}
