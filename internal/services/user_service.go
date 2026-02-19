package services

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

// CreateUserRequest for creating new staff/manager/chef
// Note: Email is optional for staff/manager/chef (they use staff_key + password)
// Email is only required for admin during registration
// StaffKey is generated on frontend and passed here (format: SK_XXXXXXXXXX - 13 chars total)
type CreateUserRequest struct {
	Name      string `json:"name" validate:"required,min=2"`
	Email     string `json:"email" validate:"omitempty,email"`
	Phone     string `json:"phone" validate:"required"`
	Password  string `json:"password" validate:"required,min=6"`
	Role      string `json:"role" validate:"required,oneof=manager staff chef"`
	StaffKey  string `json:"staff_key" validate:"required,len=13"` // Frontend-generated key (SK_XXXXXXXXXX)
}

// UpdateUserRequest for updating existing staff/manager/chef
type UpdateUserRequest struct {
	Name     string `json:"name" validate:"omitempty,min=2"`
	Phone    string `json:"phone"`
	Role     string `json:"role" validate:"omitempty,oneof=manager staff chef"`
	IsActive *bool  `json:"is_active"`
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

	// Check account limits: 1 admin, 1 manager, 2 staff, 1 chef (max 5 total)
	var userCounts struct {
		AdminCount   int64
		ManagerCount int64
		StaffCount   int64
		ChefCount    int64
	}

	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ?", restaurantID, "admin").Count(&userCounts.AdminCount)
	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ?", restaurantID, "manager").Count(&userCounts.ManagerCount)
	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ?", restaurantID, "staff").Count(&userCounts.StaffCount)
	s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ?", restaurantID, "chef").Count(&userCounts.ChefCount)

	log.Printf("📊 Current user counts for restaurant %s: Admin=%d, Manager=%d, Staff=%d, Chef=%d",
		restaurantID, userCounts.AdminCount, userCounts.ManagerCount, userCounts.StaffCount, userCounts.ChefCount)

	// Enforce limits
	if req.Role == "manager" && userCounts.ManagerCount >= 1 {
		return nil, errors.New("account limit reached: only 1 manager account is allowed per restaurant")
	}
	if req.Role == "staff" && userCounts.StaffCount >= 2 {
		return nil, errors.New("account limit reached: only 2 staff accounts are allowed per restaurant")
	}
	if req.Role == "chef" && userCounts.ChefCount >= 1 {
		return nil, errors.New("account limit reached: only 1 chef account is allowed per restaurant")
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

	// Check if staff key already exists (globally unique)
	if req.StaffKey != "" {
		var existingUser models.User
		if err := s.db.Where("staff_key = ?", req.StaffKey).First(&existingUser).Error; err == nil {
			return nil, errors.New("staff key already in use, please regenerate a new key")
		} else if err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("database error: %w", err)
		}
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("password hashing failed: %w", err)
	}

	// Create new user
	user := &models.User{
		ID:           uuid.New().String(),
		RestaurantID: restaurantID,
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hashedPassword,
		Role:         req.Role,
		StaffKey:     req.StaffKey, // Save the frontend-generated staff key
		IsActive:     true,
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

	// Apply filters
	if role, ok := filters["role"].(string); ok && role != "" {
		query = query.Where("role = ?", role)
	}

	if isActive, ok := filters["is_active"].(bool); ok {
		query = query.Where("is_active = ?", isActive)
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

	// Update fields
	updates := map[string]interface{}{}

	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.Phone != "" {
		updates["phone"] = req.Phone
	}

	if req.Role != "" && (req.Role == "manager" || req.Role == "staff") {
		updates["role"] = req.Role
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
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

	log.Printf("✅ User updated: %s (ID: %s)", user.Email, user.ID)
	return user, nil
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

	// Prevent deletion of last admin
	if user.Role == "admin" {
		var adminCount int64
		if err := s.db.Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "admin", true).Count(&adminCount).Error; err != nil {
			return fmt.Errorf("failed to check admin count: %w", err)
		}

		if adminCount <= 1 {
			return errors.New("cannot delete the only admin user. assign admin role to someone else first")
		}
	}

	// Soft delete: set is_active to false
	if err := s.db.Model(user).Update("is_active", false).Update("updated_at", time.Now()).Error; err != nil {
		log.Printf("❌ Failed to delete user: %v", err)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	log.Printf("✅ User deleted (soft): %s (ID: %s)", user.Email, userID)
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

func (s *UserService) RegenerateStaffKey(userID string) (string, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return "", err
	}

	if user.Role != "manager" && user.Role != "staff" && user.Role != "chef" {
		return "", errors.New("staff key can only be regenerated for manager/staff/chef users")
	}

	var newStaffKey string
	for i := 0; i < 10; i++ {
		candidate := "SK_" + strings.ToUpper(strings.ReplaceAll(uuid.New().String()[:10], "-", ""))

		var count int64
		if err := s.db.Model(&models.User{}).Where("staff_key = ?", candidate).Count(&count).Error; err != nil {
			return "", fmt.Errorf("failed to validate staff key uniqueness: %w", err)
		}

		if count == 0 {
			newStaffKey = candidate
			break
		}
	}

	if newStaffKey == "" {
		return "", errors.New("failed to generate unique staff key")
	}

	if err := s.db.Model(user).Updates(map[string]interface{}{
		"staff_key":        newStaffKey,
		"key_generated_at": time.Now(),
		"updated_at":       time.Now(),
	}).Error; err != nil {
		return "", fmt.Errorf("failed to regenerate staff key: %w", err)
	}

	log.Printf("✅ Staff key regenerated for user: %s", userID)
	return newStaffKey, nil
}
