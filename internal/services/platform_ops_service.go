package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
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
	// MonthlyPriceWithGST is the subscription price including 18% GST, as charged at checkout.
	MonthlyPriceWithGST int   `json:"monthly_price_with_gst"`
	AdminCount          int64 `json:"admin_count"`
	StaffCount          int64 `json:"staff_count"`
	TableCount          int64 `json:"table_count"`
	// MonthOrders counts dine-in + counter orders billed this calendar month.
	MonthOrders int64 `json:"month_orders"`
	// MonthRevenue is this month's billed revenue including GST (sum of order totals).
	MonthRevenue float64   `json:"month_revenue"`
	CreatedAt    time.Time `json:"created_at"`
	// CustomDealRequestPending is true when the restaurant asked for a negotiated plan.
	CustomDealRequestPending bool `json:"custom_deal_request_pending"`
	RequestedMaxTables       int  `json:"requested_max_tables,omitempty"`
}

type PlatformRestaurantDetail struct {
	PlatformRestaurantSummary
	Selection         SubscriptionSelection        `json:"selection"`
	Limits            SubscriptionLimits           `json:"limits"`
	Usage             SubscriptionUsage            `json:"usage"`
	HasEverPaid       bool                         `json:"has_ever_paid"`
	StartMode         string                       `json:"start_mode"`
	PricingMode       string                       `json:"pricing_mode"`
	CustomDeal        *CustomDeal                  `json:"custom_deal,omitempty"`
	CustomDealRequest *CustomDealRequest           `json:"custom_deal_request,omitempty"`
	IsSelfService     bool                         `json:"is_self_service"`
	CounterModes      string                       `json:"counter_service_modes"`
	RecentRenewals    []models.SubscriptionRenewal `json:"recent_renewals"`
	AdminLoginHint    string                       `json:"admin_login_hint,omitempty"`
}

type SetCustomDealRequest struct {
	Deal         CustomDeal `json:"deal"`
	Activate     bool       `json:"activate"`       // set phase active + has_ever_paid
	DurationDays int        `json:"duration_days"`  // optional extend/set end from now
	Reason       string     `json:"reason" validate:"required"`
}

type ClearCustomDealRequest struct {
	Reason string `json:"reason" validate:"required"`
}

type CancelCustomDealRequestRequest struct {
	Reason string `json:"reason" validate:"required"`
}

type GrantSubscriptionRequest struct {
	Selection    *SubscriptionSelection `json:"selection"`
	BillingCycle string                 `json:"billing_cycle"` // quarterly | half_yearly | annual
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

type MarkEmailVerifiedRequest struct {
	Reason string `json:"reason"`
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

func (s *PlatformOpsService) ListRestaurants(search string, phase string, customDealPending bool, limit, offset int) ([]PlatformRestaurantSummary, int64, error) {
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
		if customDealPending && !summary.CustomDealRequestPending {
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
		Selection:                 cfg.EffectiveSelection(),
		Limits:                    limits,
		Usage:                     usage,
		HasEverPaid:               cfg.HasEverPaid,
		StartMode:                 cfg.StartMode,
		PricingMode:               cfg.PricingMode,
		CustomDeal:                cfg.CustomDeal,
		CustomDealRequest:         cfg.CustomDealRequest,
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

	billingCycle := NormalizeBillingCycle(req.BillingCycle)
	if billingCycle == "" && strings.TrimSpace(req.BillingCycle) != "" {
		return nil, errors.New("billing_cycle must be quarterly, half_yearly, or annual")
	}
	if billingCycle == "" {
		billingCycle = BillingCycleQuarterly
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
	if cfg.IsCustomDeal() {
		return nil, errors.New("restaurant has a custom commercial deal; update or clear the deal instead of catalog selection")
	}
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

	cfg.Phase = phase
	cfg.Selection = validated
	cfg.Quote = quote
	cfg.HasEverPaid = hasEverPaid
	cfg.PendingSelection = nil
	cfg.PendingChangeAt = nil
	configJSON, err := MarshalSubscriptionConfig(cfg)
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

func (s *PlatformOpsService) SetCustomDeal(restaurantID string, req SetCustomDealRequest, actor string) (*models.Restaurant, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}
	deal, err := ValidateCustomDeal(req.Deal)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	deal.SetBy = actor
	deal.SetAt = &now

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	oldSnapshot, _ := json.Marshal(restaurant)
	cfg := ParseStoredSubscriptionConfig(&restaurant)
	hadPendingRequest := HasPendingCustomDealRequest(cfg)
	// Prefill empty capacity from the restaurant's pending request when ops omits it.
	if hadPendingRequest && deal.Selection.MaxTables <= 0 {
		deal.Selection = SelectionFromCustomDealRequest(*cfg.CustomDealRequest)
		deal, err = ValidateCustomDeal(deal)
		if err != nil {
			return nil, err
		}
	}
	quote := QuoteFromCustomDeal(deal)

	cfg.PricingMode = PricingModeCustom
	cfg.CustomDeal = &deal
	cfg.Selection = deal.Selection
	cfg.Quote = quote
	cfg.PendingSelection = nil
	cfg.PendingChangeAt = nil
	if cfg.CustomDealRequest != nil {
		fulfilled := *cfg.CustomDealRequest
		fulfilled.Status = CustomDealRequestFulfilled
		cfg.CustomDealRequest = &fulfilled
	}

	if req.Activate {
		cfg.Phase = SubscriptionPhaseActive
		cfg.HasEverPaid = true
		if cfg.StartMode == "" {
			cfg.StartMode = "paid"
		}
		if cfg.PeriodStartedAt == nil {
			cfg.PeriodStartedAt = &now
		}
	} else if hadPendingRequest || (!cfg.HasEverPaid && cfg.Phase != SubscriptionPhaseTrial) || IsSubscriptionAccessBlocked(&restaurant) {
		// Deal quoted — restaurant pays from app (email notifies them).
		cfg.Phase = SubscriptionPhasePendingPayment
	}

	counterModes := "both"
	isSelfService := false
	ApplyOperationModeToRestaurant(&isSelfService, &counterModes, deal.Selection.OperationMode)
	restaurant.IsSelfService = isSelfService
	restaurant.CounterServiceModes = counterModes
	restaurant.SubscriptionMonthlyPrice = deal.MonthlyPrice
	restaurant.SubscriptionPlan = "custom"

	if req.DurationDays > 0 {
		restaurant.SubscriptionEnd = time.Now().AddDate(0, 0, req.DurationDays)
	} else if req.Activate && (restaurant.SubscriptionEnd.IsZero() || restaurant.SubscriptionEnd.Before(time.Now())) {
		cycle := NormalizeBillingCycle(deal.Selection.BillingCycle)
		if cycle == "" {
			cycle = BillingCycleQuarterly
		}
		restaurant.SubscriptionEnd = NextSubscriptionEnd(time.Time{}, cycle)
	}

	configJSON, err := MarshalSubscriptionConfig(cfg)
	if err != nil {
		return nil, err
	}
	restaurant.SubscriptionConfig = configJSON

	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	s.writePlatformAudit(restaurantID, actor, "platform_set_custom_deal", reason, oldSnapshot, restaurant)
	if !req.Activate {
		s.sendCustomDealReadyEmail(&restaurant, deal, quote)
	}
	return &restaurant, nil
}

func (s *PlatformOpsService) ClearCustomDeal(restaurantID string, req ClearCustomDealRequest, actor string) (*models.Restaurant, error) {
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
	cfg := ParseStoredSubscriptionConfig(&restaurant)
	sel := cfg.Selection
	if cfg.CustomDeal != nil {
		sel = cfg.CustomDeal.Selection
	}
	validated, err := ValidateSubscriptionSelection(sel)
	if err != nil {
		validated = DefaultSubscriptionSelection()
	}
	quote := CalculateSubscriptionQuoteForTier(validated, restaurant.CityTier)

	cfg.PricingMode = PricingModeCatalog
	cfg.CustomDeal = nil
	cfg.Selection = validated
	cfg.Quote = quote

	restaurant.SubscriptionMonthlyPrice = quote.MonthlySubtotal
	restaurant.SubscriptionPlan = SubscriptionPlanFromSelection(validated)

	configJSON, err := MarshalSubscriptionConfig(cfg)
	if err != nil {
		return nil, err
	}
	restaurant.SubscriptionConfig = configJSON

	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	s.writePlatformAudit(restaurantID, actor, "platform_clear_custom_deal", reason, oldSnapshot, restaurant)
	return &restaurant, nil
}

// CancelCustomDealRequest dismisses a pending in-app custom plan review so the
// restaurant can continue with catalog / self-serve pricing. Does not clear an
// active custom deal — use ClearCustomDeal for that.
func (s *PlatformOpsService) CancelCustomDealRequest(restaurantID string, req CancelCustomDealRequestRequest, actor string) (*models.Restaurant, error) {
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
	cfg := ParseStoredSubscriptionConfig(&restaurant)
	if !HasPendingCustomDealRequest(cfg) {
		return nil, errors.New("no pending custom plan request to dismiss")
	}
	if !MarkCustomDealRequestCancelled(&cfg) {
		return nil, errors.New("no pending custom plan request to dismiss")
	}

	configJSON, err := MarshalSubscriptionConfig(cfg)
	if err != nil {
		return nil, err
	}
	restaurant.SubscriptionConfig = configJSON

	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	s.writePlatformAudit(restaurantID, actor, "platform_cancel_custom_deal_request", reason, oldSnapshot, restaurant)
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

// MarkEmailVerified manually sets is_email_verified so ops can unblock approval
// when outbound SMTP is unavailable (e.g. provider blocks mail ports).
func (s *PlatformOpsService) MarkEmailVerified(restaurantID string, req MarkEmailVerifiedRequest, actor string) (*models.Restaurant, error) {
	restaurantID = strings.TrimSpace(restaurantID)
	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	if strings.TrimSpace(restaurant.Email) == "" {
		return nil, errors.New("restaurant has no registered email")
	}
	if restaurant.IsEmailVerified {
		return nil, errors.New("restaurant email is already verified")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "Ops manually marked email verified (SMTP unavailable)"
	}

	oldSnapshot, _ := json.Marshal(restaurant)
	restaurant.IsEmailVerified = true
	if err := s.db.Save(&restaurant).Error; err != nil {
		return nil, err
	}

	// Invalidate unused verification tokens so old links cannot be reused.
	_ = s.db.Model(&models.EmailVerification{}).
		Where("restaurant_id = ? AND email = ? AND is_used = false", restaurant.ID, restaurant.Email).
		Update("is_used", true).Error

	s.writePlatformAudit(restaurantID, actor, "platform_mark_email_verified", reason, oldSnapshot, restaurant)
	return &restaurant, nil
}

// PasswordResetLinkResult is returned to platform so ops can email the link manually.
type PasswordResetLinkResult struct {
	ResetLink string    `json:"reset_link"`
	Email     string    `json:"email"`
	LoginID   string    `json:"login_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IssuePasswordResetLink creates a password reset URL for the restaurant admin
// without sending email (ops copy/paste from hello@thebillgenie.com when SMTP is down).
func (s *PlatformOpsService) IssuePasswordResetLink(restaurantID, actor, reason string) (*PasswordResetLinkResult, error) {
	restaurantID = strings.TrimSpace(restaurantID)
	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("restaurant not found")
		}
		return nil, err
	}

	var admin models.User
	if err := s.db.Where("restaurant_id = ? AND role = ? AND is_active = ?", restaurant.ID, "admin", true).
		Order("created_at ASC").First(&admin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("no active admin user found for this restaurant")
		}
		return nil, err
	}
	if strings.TrimSpace(admin.Email) == "" {
		return nil, errors.New("admin user has no email on file")
	}

	auth := NewAuthService(s.db, "", "")
	link, expiresAt, err := auth.IssueAdminPasswordResetLink(&admin)
	if err != nil {
		return nil, err
	}

	auditReason := strings.TrimSpace(reason)
	if auditReason == "" {
		auditReason = "Ops issued password reset link for manual email delivery"
	}
	oldSnapshot, _ := json.Marshal(map[string]interface{}{
		"admin_user_id": admin.ID,
		"admin_email":   admin.Email,
		"login_id":      admin.StaffKey,
	})
	s.writePlatformAudit(restaurantID, actor, "platform_issue_password_reset_link", auditReason, oldSnapshot, restaurant)

	return &PasswordResetLinkResult{
		ResetLink: link,
		Email:     admin.Email,
		LoginID:   admin.StaffKey,
		ExpiresAt: expiresAt,
	}, nil
}

// ResendVerificationEmail sends a fresh verification link to the restaurant's
// registered email (ops recovery when the original mail was missed or expired).
func (s *PlatformOpsService) ResendVerificationEmail(restaurantID, actor, reason string) (string, error) {
	restaurantID = strings.TrimSpace(restaurantID)
	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", restaurantID).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", errors.New("restaurant not found")
		}
		return "", err
	}

	email := strings.TrimSpace(restaurant.Email)
	if email == "" {
		return "", errors.New("restaurant has no registered email")
	}

	auth := NewAuthService(s.db, "", "")
	if err := auth.ResendVerificationEmail(restaurant.ID, email); err != nil {
		return "", err
	}

	auditReason := strings.TrimSpace(reason)
	if auditReason == "" {
		auditReason = "Ops resent email verification link"
	}
	oldSnapshot, _ := json.Marshal(map[string]interface{}{
		"email":              email,
		"was_email_verified": restaurant.IsEmailVerified,
	})
	s.writePlatformAudit(restaurantID, actor, "platform_resend_verification_email", auditReason, oldSnapshot, restaurant)

	msg := fmt.Sprintf("Verification email sent to %s", email)
	if restaurant.IsEmailVerified {
		msg += " (restaurant was already marked verified; new link issued anyway)"
	}
	return msg, nil
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

func (s *PlatformOpsService) sendCustomDealReadyEmail(
	restaurant *models.Restaurant,
	deal CustomDeal,
	quote SubscriptionQuote,
) {
	if restaurant == nil || restaurant.Email == "" {
		return
	}

	cycle := NormalizeBillingCycle(deal.Selection.BillingCycle)
	if cycle == "" {
		cycle = BillingCycleQuarterly
	}
	periodLabel := BillingCycleLabel(cycle)
	subtotal := PeriodSubtotalINR(quote, cycle)
	gst := int(math.Round(float64(subtotal) * 0.18))
	total := subtotal + gst
	tables := deal.Selection.MaxTables

	subject := "Your BillGenie custom plan is ready — open the app to pay"
	body := fmt.Sprintf(
		"Hi %s,\n\n"+
			"Good news — the BillGenie team has prepared a custom plan for %s.\n\n"+
			"Plan summary:\n"+
			"• Up to %d tables\n"+
			"• ₹%s per %s (₹%s + 18%% GST)\n\n"+
			"Open the BillGenie app (or web), go to More / Subscription, and complete payment with Razorpay to activate this plan.\n\n"+
			"If you have questions, reply to this email or contact hello@thebillgenie.com.\n\n"+
			"- BillGenie",
		restaurant.OwnerName,
		restaurant.Name,
		tables,
		formatINR(total),
		periodLabel,
		formatINR(subtotal),
	)

	if err := sendEmailSMTP(restaurant.Email, subject, body); err != nil {
		log.Printf("⚠️  Failed to send custom deal ready email to %s: %v", restaurant.Email, err)
	}
}

func formatINR(amount int) string {
	return fmt.Sprintf("%d", amount)
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
	if err := deleteWhere(&models.MenuItemVariant{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete menu item variants: %w", err)
	}
	if err := deleteWhere(&models.MenuItem{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete menu items: %w", err)
	}
	if err := deleteWhere(&models.StockExpenditure{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete stock expenditures: %w", err)
	}
	if err := deleteWhere(&models.Expense{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete expenses: %w", err)
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
	// Support issues + audit logs reference users (fk_support_issues_user / user_id).
	// Delete them before users or restaurant delete fails with SQLSTATE 23503.
	if err := deleteWhere(&models.SupportIssue{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete support issues: %w", err)
	}
	if len(userIDs) > 0 {
		if err := deleteWhere(&models.SupportIssue{}, "user_id IN ?", userIDs); err != nil {
			return fmt.Errorf("delete support issues by user: %w", err)
		}
	}
	if err := deleteWhere(&models.AuditLog{}, "restaurant_id = ?", restaurantID); err != nil {
		return fmt.Errorf("delete audit logs: %w", err)
	}
	if len(userIDs) > 0 {
		if err := deleteWhere(&models.AuditLog{}, "user_id IN ?", userIDs); err != nil {
			return fmt.Errorf("delete audit logs by user: %w", err)
		}
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

	monthOrders, monthRevenue := s.monthOrderStats(r.ID)

	pendingRequest := HasPendingCustomDealRequest(cfg)
	requestedTables := 0
	if pendingRequest {
		requestedTables = cfg.CustomDealRequest.MaxTables
	}

	return PlatformRestaurantSummary{
		ID:                       r.ID,
		RestaurantCode:           r.RestaurantCode,
		Name:                     r.Name,
		OwnerName:                r.OwnerName,
		Email:                    r.Email,
		Phone:                    r.Phone,
		City:                     r.City,
		SubscriptionPlan:         r.SubscriptionPlan,
		SubscriptionPhase:        cfg.Phase,
		SubscriptionEnd:          r.SubscriptionEnd,
		DaysRemaining:            daysRemaining,
		IsActive:                 r.IsActive,
		IsAccessBlocked:          IsSubscriptionAccessBlocked(r),
		IsEmailVerified:          r.IsEmailVerified,
		IsApproved:               r.IsApproved,
		MonthlyPrice:             r.SubscriptionMonthlyPrice,
		MonthlyPriceWithGST:      SubscriptionPriceWithGST(r.SubscriptionMonthlyPrice),
		AdminCount:               adminCount,
		StaffCount:               staffCount,
		TableCount:               tableCount,
		MonthOrders:              monthOrders,
		MonthRevenue:             monthRevenue,
		CreatedAt:                r.CreatedAt,
		CustomDealRequestPending: pendingRequest,
		RequestedMaxTables:       requestedTables,
	}
}

// SubscriptionPriceWithGST returns the subscription amount including the 18% GST
// applied at checkout (see subscriptionGSTPercent in subscription_renewal_service.go).
func SubscriptionPriceWithGST(subtotal int) int {
	if subtotal <= 0 {
		return 0
	}
	gst := int(math.Round(float64(subtotal) * subscriptionGSTPercent / 100))
	return subtotal + gst
}

// monthOrderStats aggregates this calendar month's billed orders (dine-in and counter
// combined) using the same filter as tenant sales reports; revenue includes GST.
func (s *PlatformOpsService) monthOrderStats(restaurantID string) (int64, float64) {
	now := time.Now().In(RestaurantLocation())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var result struct {
		MonthOrders  int64
		MonthRevenue float64
	}
	err := s.db.Model(&models.Order{}).
		Where("restaurant_id = ?", restaurantID).
		Where("(status = ? OR (order_type = ? AND payment_method <> ''))", "completed", "counter").
		Where(historyActivityAtSQL+" >= ?", monthStart).
		Select("COUNT(*) AS month_orders, COALESCE(SUM(total), 0) AS month_revenue").
		Scan(&result).Error
	if err != nil {
		return 0, 0
	}
	return result.MonthOrders, result.MonthRevenue
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
		UserID:       actor, // platform actor label (not a users.id UUID)
		Action:       action,
		Entity:       "restaurant",
		EntityID:     restaurantID,
		OldValues:    json.RawMessage(oldSnapshot),
		NewValues:    json.RawMessage(newSnapshot),
	}
	_ = s.db.Create(&entry).Error
}

// PlatformAuditEntry is a platform-console audit row.
type PlatformAuditEntry struct {
	ID           string          `json:"id"`
	RestaurantID string          `json:"restaurant_id"`
	Actor        string          `json:"actor"`
	Action       string          `json:"action"`
	Entity       string          `json:"entity"`
	EntityID     string          `json:"entity_id"`
	OldValues    json.RawMessage `json:"old_values,omitempty"`
	NewValues    json.RawMessage `json:"new_values,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// ListPlatformAuditLogs returns recent platform_* audit rows for ops review.
func (s *PlatformOpsService) ListPlatformAuditLogs(restaurantID string, limit, offset int) ([]PlatformAuditEntry, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	q := s.db.Model(&models.AuditLog{}).Where("action LIKE ?", "platform_%")
	if restaurantID = strings.TrimSpace(restaurantID); restaurantID != "" {
		q = q.Where("restaurant_id = ?", restaurantID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.AuditLog
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]PlatformAuditEntry, 0, len(rows))
	for _, row := range rows {
		actor := row.UserID
		if actor == "" {
			actor = "platform_ops"
		}
		out = append(out, PlatformAuditEntry{
			ID:           row.ID,
			RestaurantID: row.RestaurantID,
			Actor:        actor,
			Action:       row.Action,
			Entity:       row.Entity,
			EntityID:     row.EntityID,
			OldValues:    row.OldValues,
			NewValues:    row.NewValues,
			CreatedAt:    row.CreatedAt,
		})
	}
	return out, total, nil
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
