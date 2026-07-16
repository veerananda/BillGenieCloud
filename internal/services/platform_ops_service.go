package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlatformRestaurantSummary struct {
	ID                string    `json:"id"`
	RestaurantCode    string    `json:"restaurant_code"`
	Name              string    `json:"name"`
	OwnerName         string    `json:"owner_name"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	City              string    `json:"city"`
	SubscriptionPlan  string    `json:"subscription_plan"`
	SubscriptionPhase string    `json:"subscription_phase"`
	SubscriptionEnd   time.Time `json:"subscription_end"`
	DaysRemaining     int       `json:"days_remaining"`
	IsActive          bool      `json:"is_active"`
	IsAccessBlocked   bool      `json:"is_access_blocked"`
	IsEmailVerified   bool      `json:"is_email_verified"`
	IsApproved        bool      `json:"is_approved"`
	MonthlyPrice      int       `json:"monthly_price"`
	AdminCount        int64     `json:"admin_count"`
	StaffCount        int64     `json:"staff_count"`
	TableCount        int64     `json:"table_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type PlatformRestaurantDetail struct {
	PlatformRestaurantSummary
	Selection      SubscriptionSelection        `json:"selection"`
	Limits         SubscriptionLimits           `json:"limits"`
	Usage          SubscriptionUsage            `json:"usage"`
	HasEverPaid    bool                         `json:"has_ever_paid"`
	StartMode      string                       `json:"start_mode"`
	IsSelfService  bool                         `json:"is_self_service"`
	CounterModes   string                       `json:"counter_service_modes"`
	RecentRenewals []models.SubscriptionRenewal `json:"recent_renewals"`
	AdminLoginHint string                       `json:"admin_login_hint,omitempty"`
}

type GrantSubscriptionRequest struct {
	Selection    *SubscriptionSelection `json:"selection"`
	BillingCycle string                 `json:"billing_cycle"` // monthly | annual
	DurationDays int                    `json:"duration_days"` // 0 = default (30d / 365d)
	Reason       string                 `json:"reason" validate:"required"`
}

type ExtendTrialRequest struct {
	Days   int    `json:"days"`
	Reason string `json:"reason" validate:"required"`
}

type UpdateSelectionRequest struct {
	Selection SubscriptionSelection `json:"selection"`
	Reason    string                `json:"reason" validate:"required"`
}

type SetActiveRequest struct {
	IsActive bool   `json:"is_active"`
	Reason   string `json:"reason" validate:"required"`
}

type SetApprovedRequest struct {
	Reason string `json:"reason" validate:"required"`
}

type DeleteRestaurantRequest struct {
	Reason      string `json:"reason" validate:"required"`
	ConfirmName string `json:"confirm_name" validate:"required"`
}

type PlatformOpsService struct {
	db             *gorm.DB
	renewalService *SubscriptionRenewalService
}

func NewPlatformOpsService(db *gorm.DB) *PlatformOpsService {
	return &PlatformOpsService{
		db:             db,
		renewalService: NewSubscriptionRenewalService(db),
	}
}

func (s *PlatformOpsService) ListRestaurants(search string, phase string, limit, offset int) ([]PlatformRestaurantSummary, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := s.db.Model(&models.Restaurant{})
	search = strings.TrimSpace(search)
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(city) LIKE ? OR LOWER(restaurant_code) LIKE ?",
			like, like, like, like,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var restaurants []models.Restaurant
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&restaurants).Error; err != nil {
		return nil, 0, err
	}

	summaries := make([]PlatformRestaurantSummary, 0, len(restaurants))
	for i := range restaurants {
		summary := s.buildSummary(&restaurants[i])
		if phase != "" && summary.SubscriptionPhase != phase {
			continue
		}
		summaries = append(summaries, summary)
	}

	return summaries, total, nil
}

func (s *PlatformOpsService) GetRestaurant(restaurantID string) (*PlatformRestaurantDetail, error) {
	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	summary := s.buildSummary(&restaurant)
	cfg := ParseStoredSubscriptionConfig(&restaurant)
	limits, _ := LoadSubscriptionLimits(s.db, &restaurant)
	usage, _ := s.loadUsage(restaurant.ID)

	var renewals []models.SubscriptionRenewal
	_ = s.db.Where("restaurant_id = ?", restaurantID).
		Order("created_at DESC").Limit(10).Find(&renewals).Error

	adminHint := ""
	var admin models.User
	if err := s.db.Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "admin", true).
		Order("created_at ASC").First(&admin).Error; err == nil {
		if admin.StaffKey != "" {
			adminHint = maskLoginHint(admin.StaffKey)
		}
	}

	return &PlatformRestaurantDetail{
		PlatformRestaurantSummary: summary,
		Selection:                 cfg.Selection,
		Limits:                    limits,
		Usage:                     usage,
		HasEverPaid:               cfg.HasEverPaid,
		StartMode:                 cfg.StartMode,
		IsSelfService:             restaurant.IsSelfService,
		CounterModes:              restaurant.CounterServiceModes,
		RecentRenewals:            renewals,
		AdminLoginHint:            adminHint,
	}, nil
}

func (s *PlatformOpsService) GrantSubscription(restaurantID string, req GrantSubscriptionRequest, actor string) (*models.Restaurant, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	cfg := ParseStoredSubscriptionConfig(&restaurant)
	selection := cfg.Selection
	if req.Selection != nil {
		validated, err := ValidateSubscriptionSelection(*req.Selection)
		if err != nil {
			return nil, err
		}
		selection = validated
	}

	billingCycle := strings.TrimSpace(req.BillingCycle)
	if billingCycle == "" {
		billingCycle = "monthly"
	}
	if billingCycle != "monthly" && billingCycle != "annual" {
		return nil, errors.New("billing_cycle must be monthly or annual")
	}

	oldSnapshot, _ := json.Marshal(restaurant)

	if err := s.renewalService.applyPaidSelection(&restaurant, cfg, selection, billingCycle); err != nil {
		return nil, err
	}

	// applyPaidSelection already preserves unused days; only override when an explicit duration is requested.
	if req.DurationDays > 0 {
		restaurant.SubscriptionEnd = time.Now().AddDate(0, 0, req.DurationDays)
	}

	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	_ = s.trialServiceMarkConverted(restaurantID)
	s.writePlatformAudit(restaurantID, actor, "platform_grant_subscription", reason, oldSnapshot, restaurant)

	return &restaurant, nil
}

func (s *PlatformOpsService) ExtendTrial(restaurantID string, req ExtendTrialRequest, actor string) (*models.Restaurant, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}
	days := req.Days
	if days <= 0 {
		days = TrialDurationDays
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	oldSnapshot, _ := json.Marshal(restaurant)
	cfg := ParseStoredSubscriptionConfig(&restaurant)
	selection := cfg.Selection
	if selection.OperationMode == "" {
		selection = FixedTrialSelection()
	}
	quote := CalculateSubscriptionQuote(selection)

	restaurant.SubscriptionEnd = time.Now().AddDate(0, 0, days)
	restaurant.SubscriptionPlan = "trial"
	restaurant.SubscriptionMonthlyPrice = quote.MonthlySubtotal

	configJSON, err := BuildSubscriptionConfigJSON(SubscriptionPhaseTrial, "trial", selection, quote, cfg.HasEverPaid)
	if err != nil {
		return nil, err
	}
	restaurant.SubscriptionConfig = configJSON

	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	s.writePlatformAudit(restaurantID, actor, "platform_extend_trial", reason, oldSnapshot, restaurant)
	return &restaurant, nil
}

func (s *PlatformOpsService) UpdateSelection(restaurantID string, req UpdateSelectionRequest, actor string) (*models.Restaurant, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}

	validated, err := ValidateSubscriptionSelection(req.Selection)
	if err != nil {
		return nil, err
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	oldSnapshot, _ := json.Marshal(restaurant)
	cfg := ParseStoredSubscriptionConfig(&restaurant)
	quote := CalculateSubscriptionQuote(validated)

	counterModes := "both"
	isSelfService := restaurant.IsSelfService
	ApplyOperationModeToRestaurant(&isSelfService, &counterModes, validated.OperationMode)

	restaurant.IsSelfService = isSelfService
	restaurant.CounterServiceModes = counterModes
	restaurant.SubscriptionMonthlyPrice = quote.MonthlySubtotal
	restaurant.SubscriptionPlan = SubscriptionPlanFromSelection(validated)

	phase := cfg.Phase
	if phase == "" {
		phase = SubscriptionPhaseActive
	}
	hasEverPaid := cfg.HasEverPaid
	if phase == SubscriptionPhaseActive {
		hasEverPaid = true
	}

	configJSON, err := BuildSubscriptionConfigJSON(phase, cfg.StartMode, validated, quote, hasEverPaid)
	if err != nil {
		return nil, err
	}
	restaurant.SubscriptionConfig = configJSON

	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	s.writePlatformAudit(restaurantID, actor, "platform_update_selection", reason, oldSnapshot, restaurant)
	return &restaurant, nil
}

func (s *PlatformOpsService) SetActive(restaurantID string, req SetActiveRequest, actor string) (*models.Restaurant, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	oldSnapshot, _ := json.Marshal(restaurant)
	restaurant.IsActive = req.IsActive
	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	action := "platform_suspend"
	if req.IsActive {
		action = "platform_reactivate"
	}
	s.writePlatformAudit(restaurantID, actor, action, reason, oldSnapshot, restaurant)
	return &restaurant, nil
}

// ApproveRestaurant marks a restaurant as approved to sign in, after BillGenie
// staff review. Requires the restaurant's email to already be verified, and
// notifies the restaurant owner by email once approved.
func (s *PlatformOpsService) ApproveRestaurant(restaurantID string, req SetApprovedRequest, actor string) (*models.Restaurant, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	if !restaurant.IsEmailVerified {
		return nil, errors.New("restaurant must verify their email before approval")
	}
	if restaurant.IsApproved {
		return nil, errors.New("restaurant is already approved")
	}

	oldSnapshot, _ := json.Marshal(restaurant)
	restaurant.IsApproved = true
	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	s.writePlatformAudit(restaurantID, actor, "platform_approve_restaurant", reason, oldSnapshot, restaurant)
	s.sendApprovalEmail(&restaurant)
	return &restaurant, nil
}

func (s *PlatformOpsService) sendApprovalEmail(restaurant *models.Restaurant) {
	if restaurant.Email == "" {
		return
	}

	loginHint := ""
	var admin models.User
	if err := s.db.Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurant.ID, "admin", true).
		Order("created_at ASC").First(&admin).Error; err == nil {
		loginHint = admin.StaffKey
	}

	subject := "Your BillGenie account has been approved"
	body := fmt.Sprintf(
		"Hi %s,\n\nGood news - %s has been reviewed and approved by the BillGenie team.\n"+
			"Your email is verified and your account is now fully active.\n\n"+
			"Sign in now with your login number: %s\n\n- BillGenie",
		restaurant.OwnerName, restaurant.Name, loginHint,
	)

	if err := sendEmailSMTP(restaurant.Email, subject, body); err != nil {
		log.Printf("⚠️  Failed to send approval email to %s: %v", restaurant.Email, err)
	}
}

// DeleteRestaurant permanently removes a tenant and all related rows.
// Trial eligibility (email/phone) is retained so a deleted account cannot claim another free trial.
func (s *PlatformOpsService) DeleteRestaurant(restaurantID string, req DeleteRestaurantRequest, actor string) error {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return errors.New("reason is required")
	}
	confirmName := strings.TrimSpace(req.ConfirmName)
	if confirmName == "" {
		return errors.New("confirm_name is required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("restaurant not found")
		}
		return err
	}

	if !strings.EqualFold(confirmName, strings.TrimSpace(restaurant.Name)) {
		return errors.New("confirm_name does not match restaurant name")
	}

	snapshot, _ := json.Marshal(map[string]interface{}{
		"restaurant": restaurant,
		"reason":     reason,
		"actor":      actor,
	})
	log.Printf("platform_delete_restaurant: id=%s name=%q actor=%q reason=%q snapshot=%s",
		restaurantID, restaurant.Name, actor, reason, string(snapshot))

	tx := s.db.Begin()

	deleteWhere := func(model interface{}, query string, args ...interface{}) error {
		if err := tx.Where(query, args...).Delete(model).Error; err != nil {
			tx.Rollback()
			return err
		}
		return nil
	}

	var userIDs []string
	if err := tx.Model(&models.User{}).Where("restaurant_id = ?", restaurantID).Pluck("id", &userIDs).Error; err != nil {
		tx.Rollback()
		return err
	}

	var orderIDs []string
	if err := tx.Model(&models.Order{}).Where("restaurant_id = ?", restaurantID).Pluck("id", &orderIDs).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&models.RestaurantTable{}).
		Where("restaurant_id = ?", restaurantID).
		Update("current_order_id", nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(orderIDs) > 0 {
		if err := deleteWhere(&models.OrderItem{}, "order_id IN ?", orderIDs); err != nil {
			return fmt.Errorf("delete order items: %w", err)
		}
	}

	if err := deleteWhere(&models.Transaction{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete transactions: %w", err)
	}
	if err := deleteWhere(&models.Order{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete orders: %w", err)
	}
	if err := deleteWhere(&models.Inventory{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete inventory: %w", err)
	}
	if err := deleteWhere(&models.MenuItemIngredient{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete menu item ingredients: %w", err)
	}
	if err := deleteWhere(&models.MenuItem{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete menu items: %w", err)
	}
	if err := deleteWhere(&models.Ingredient{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete ingredients: %w", err)
	}

	if len(userIDs) > 0 {
		if err := deleteWhere(&models.RefreshToken{}, "user_id IN ?", userIDs); err != nil {
			return fmt.Errorf("delete refresh tokens: %w", err)
		}
		if err := deleteWhere(&models.PasswordReset{}, "user_id IN ?", userIDs); err != nil {
			return fmt.Errorf("delete password resets: %w", err)
		}
		if err := deleteWhere(&models.LoginRecoveryOTP{}, "user_id IN ?", userIDs); err != nil {
			return fmt.Errorf("delete login recovery otps: %w", err)
		}
	}

	if err := deleteWhere(&models.UserSession{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	if err := deleteWhere(&models.User{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete users: %w", err)
	}
	if err := deleteWhere(&models.RestaurantTable{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete tables: %w", err)
	}
	if err := deleteWhere(&models.EmailVerification{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete email verifications: %w", err)
	}
	if err := deleteWhere(&models.SubscriptionRenewal{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete subscription renewals: %w", err)
	}
	// Keep trial_eligibilities rows so the same email/phone cannot claim another free trial
	// after the restaurant is deleted.
	if err := deleteWhere(&models.SupportIssue{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete support issues: %w", err)
	}
	if err := deleteWhere(&models.AuditLog{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete audit logs: %w", err)
	}
	if err := tx.Delete(&restaurant).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("delete restaurant: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit delete restaurant: %w", err)
	}

	log.Printf("platform_delete_restaurant: completed id=%s name=%q", restaurantID, restaurant.Name)
	return nil
}

func (s *PlatformOpsService) buildSummary(r *models.Restaurant) PlatformRestaurantSummary {
	cfg := ParseStoredSubscriptionConfig(r)
	daysRemaining := int(time.Until(r.SubscriptionEnd).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	var adminCount, staffCount, tableCount int64
	_ = s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", r.ID, "admin", true).Count(&adminCount).Error
	_ = s.db.Model(&models.User{}).Where("restaurant_id = ? AND role IN ? AND is_active = ?", r.ID, []string{"manager", "staff", "chef"}, true).Count(&staffCount).Error
	_ = s.db.Model(&models.RestaurantTable{}).Where("restaurant_id = ?", r.ID).Count(&tableCount).Error

	return PlatformRestaurantSummary{
		ID:                r.ID,
		RestaurantCode:    r.RestaurantCode,
		Name:              r.Name,
		OwnerName:         r.OwnerName,
		Email:             r.Email,
		Phone:             r.Phone,
		City:              r.City,
		SubscriptionPlan:  r.SubscriptionPlan,
		SubscriptionPhase: cfg.Phase,
		SubscriptionEnd:   r.SubscriptionEnd,
		DaysRemaining:     daysRemaining,
		IsActive:          r.IsActive,
		IsAccessBlocked:   IsSubscriptionAccessBlocked(r),
		IsEmailVerified:   r.IsEmailVerified,
		IsApproved:        r.IsApproved,
		MonthlyPrice:      r.SubscriptionMonthlyPrice,
		AdminCount:        adminCount,
		StaffCount:        staffCount,
		TableCount:        tableCount,
		CreatedAt:         r.CreatedAt,
	}
}

func (s *PlatformOpsService) loadUsage(restaurantID string) (SubscriptionUsage, error) {
	var usage SubscriptionUsage
	_ = s.db.Model(&models.RestaurantTable{}).Where("restaurant_id = ?", restaurantID).Count(&usage.Tables).Error
	_ = s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "manager", true).Count(&usage.Managers).Error
	_ = s.db.Model(&models.User{}).Where("restaurant_id = ? AND role IN ? AND is_active = ?", restaurantID, []string{"staff", "chef"}, true).Count(&usage.StaffAndChefs).Error
	_ = s.db.Model(&models.User{}).Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurantID, "admin", true).Count(&usage.Admins).Error
	return usage, nil
}

func (s *PlatformOpsService) trialServiceMarkConverted(restaurantID string) error {
	return NewTrialEligibilityService(s.db).MarkConverted(restaurantID)
}

func (s *PlatformOpsService) writePlatformAudit(restaurantID, actor, action, reason string, oldSnapshot []byte, restaurant models.Restaurant) {
	newSnapshot, _ := json.Marshal(map[string]interface{}{
		"restaurant": restaurant,
		"reason":     reason,
		"actor":      actor,
	})
	entry := models.AuditLog{
		ID:           uuid.New().String(),
		RestaurantID: restaurantID,
		Action:       action,
		Entity:       "restaurant",
		EntityID:     restaurantID,
		OldValues:    json.RawMessage(oldSnapshot),
		NewValues:    json.RawMessage(newSnapshot),
	}
	_ = s.db.Create(&entry).Error
}

func maskLoginHint(staffKey string) string {
	staffKey = strings.TrimSpace(staffKey)
	if len(staffKey) <= 4 {
		return "****"
	}
	return fmt.Sprintf("****%s", staffKey[len(staffKey)-4:])
}

// BuildSummaryPublic returns a tenant summary after mutating operations.
func (s *PlatformOpsService) BuildSummaryPublic(restaurant *models.Restaurant) PlatformRestaurantSummary {
	if restaurant == nil {
		return PlatformRestaurantSummary{}
	}
	return s.buildSummary(restaurant)
}
