package services

import (
	"errors"
	"fmt"
	"math/rand"
	"net/smtp"
	"os"
	"regexp"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	loginStaffKeyPattern = regexp.MustCompile(`^\d{6}$`)
	loginAdminKeyPattern = regexp.MustCompile(`^100\d{5}$`)
)

type AuthService struct {
	db        *gorm.DB
	jwtSecret string
}

type TokenClaims struct {
	UserID       string `json:"user_id"`
	RestaurantID string `json:"restaurant_id"`
	Role         string `json:"role"`
	jwt.RegisteredClaims
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RestaurantID string `json:"restaurant_id"`
	UserID       string `json:"user_id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	CanCancelOrders     bool   `json:"can_cancel_orders"`
	CanRestockInventory bool   `json:"can_restock_inventory"`
	MenuManagementAccess bool  `json:"menu_management_access"`
}

type RegisterRequest struct {
	StartMode      string `json:"start_mode" validate:"required,oneof=trial paid"`
	RestaurantName string `json:"restaurant_name" validate:"required"`
	OwnerName      string `json:"owner_name" validate:"required"`
	Email          string `json:"email" validate:"required,email"`
	Phone          string `json:"phone" validate:"required"`
	Password       string `json:"password" validate:"required,min=8"`
	LoginID        string `json:"login_id" validate:"required"`
	Address        string `json:"address"`
	City           string `json:"city"`
	Cuisine        string `json:"cuisine"`
	Subscription   *SubscriptionSelection `json:"subscription"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // 8-digit admin login number or staff key
	Password   string `json:"password" validate:"required"`
}

// NewAuthService creates a new auth service
func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

// Register creates a new restaurant and admin user
func (s *AuthService) Register(req RegisterRequest) (*models.Restaurant, *models.User, error) {
	// Check if email already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, nil, errors.New("email already registered")
	} else if err != gorm.ErrRecordNotFound {
		return nil, nil, err
	}

	if err := ValidateAccountPassword(req.Password, ""); err != nil {
		return nil, nil, err
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, nil, err
	}

	// Generate unique restaurant code (6 characters: uppercase letters and numbers)
	restaurantCode := generateRestaurantCode(req.RestaurantName)

	// Ensure code is unique
	for {
		var existing models.Restaurant
		if err := s.db.Where("restaurant_code = ?", restaurantCode).First(&existing).Error; err == gorm.ErrRecordNotFound {
			break // Code is unique
		}
		// If code exists, add random suffix
		restaurantCode = generateRestaurantCode(req.RestaurantName)
	}

	startMode := strings.ToLower(strings.TrimSpace(req.StartMode))
	if startMode != "trial" && startMode != "paid" {
		return nil, nil, errors.New("start_mode must be trial or paid")
	}

	trialService := NewTrialEligibilityService(s.db)
	if startMode == "trial" {
		if err := trialService.EnsureTrialAvailable(req.Email, req.Phone); err != nil {
			return nil, nil, err
		}
	} else if req.Subscription == nil {
		return nil, nil, errors.New("subscription plan is required for paid registration")
	}

	// Create restaurant with trial or pending paid subscription
	var (
		subSelection SubscriptionSelection
		phase        string
		subscriptionEnd time.Time
		hasEverPaid  bool
	)

	if startMode == "trial" {
		subSelection = FixedTrialSelection()
		phase = SubscriptionPhaseTrial
		subscriptionEnd = time.Now().AddDate(0, 0, TrialDurationDays)
		hasEverPaid = false
	} else {
		validated, err := ValidateSubscriptionSelection(*req.Subscription)
		if err != nil {
			return nil, nil, err
		}
		subSelection = validated
		phase = SubscriptionPhasePendingPayment
		subscriptionEnd = time.Now()
		hasEverPaid = false
	}

	quote := CalculateSubscriptionQuote(subSelection)
	subConfig, err := BuildSubscriptionConfigJSON(phase, startMode, subSelection, quote, hasEverPaid)
	if err != nil {
		return nil, nil, err
	}

	counterModes := "both"
	isSelfService := false
	ApplyOperationModeToRestaurant(&isSelfService, &counterModes, subSelection.OperationMode)

	subscriptionPlan := "trial"
	if startMode == "paid" {
		subscriptionPlan = SubscriptionPlanFromSelection(subSelection)
	}

	restaurant := &models.Restaurant{
		ID:                       uuid.New().String(),
		RestaurantCode:           restaurantCode,
		Name:                     req.RestaurantName,
		OwnerName:                req.OwnerName,
		Email:                    req.Email,
		Phone:                    req.Phone,
		Address:                  req.Address,
		City:                     req.City,
		Cuisine:                  req.Cuisine,
		IsActive:                 true,
		IsSelfService:            isSelfService,
		CounterServiceModes:      counterModes,
		SubscriptionEnd:          subscriptionEnd,
		SubscriptionPlan:         subscriptionPlan,
		SubscriptionMonthlyPrice: quote.MonthlySubtotal,
		SubscriptionConfig:       subConfig,
	}

	if err := s.db.Create(restaurant).Error; err != nil {
		return nil, nil, err
	}

	if startMode == "trial" {
		if err := trialService.RecordTrialGrant(restaurant.ID, req.Email, req.Phone, subscriptionEnd); err != nil {
			s.db.Delete(restaurant)
			return nil, nil, fmt.Errorf("failed to record trial eligibility: %w", err)
		}
	}

	// Validate and reserve admin login number (stored as staff_key)
	loginID := strings.TrimSpace(req.LoginID)
	if !loginAdminKeyPattern.MatchString(loginID) {
		return nil, nil, errors.New("login number must be an 8-digit number starting with 100")
	}
	var existingLogin models.User
	if err := s.db.Where("staff_key = ?", loginID).First(&existingLogin).Error; err == nil {
		return nil, nil, errors.New("login number already in use — please regenerate and try again")
	} else if err != gorm.ErrRecordNotFound {
		return nil, nil, err
	}

	// Create admin user with client-chosen login number
	user := &models.User{
		ID:           uuid.New().String(),
		RestaurantID: restaurant.ID,
		Name:         req.OwnerName,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hashedPassword,
		Role:         "admin",
		IsActive:     true,
		StaffKey:     loginID,
	}

	if err := s.db.Create(user).Error; err != nil {
		// Rollback restaurant creation if user creation fails
		s.db.Delete(restaurant)
		return nil, nil, err
	}

	return restaurant, user, nil
}

// SendVerificationEmail sends an email verification token
func (s *AuthService) SendVerificationEmail(restaurantID, email string) (string, error) {
	// Generate 32-character token
	token := generateRandomToken(32)

	// Create email verification record (valid for 24 hours)
	verification := &models.EmailVerification{
		ID:           uuid.New().String(),
		RestaurantID: restaurantID,
		Email:        email,
		Token:        token,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		IsUsed:       false,
	}

	if err := s.db.Create(verification).Error; err != nil {
		return "", err
	}

	publicBase := publicAppBaseURL()
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", publicBase, token)

	subject := "Verify your BillGenie email"
	body := fmt.Sprintf(
		"Hi,\n\nPlease verify your email by opening this link:\n%s\n\n"+
			"This link expires in 24 hours. You must verify before you can sign in.\n\n- BillGenie",
		verificationLink,
	)

	if err := sendEmailSMTP(email, subject, body); err != nil {
		fmt.Printf("❌ Failed to send verification email to %s: %v\n", email, err)
		return verificationLink, err
	}

	fmt.Printf("✅ Verification email sent to %s\n", email)
	return verificationLink, nil
}

// VerifyEmail verifies an email with the provided token
func (s *AuthService) VerifyEmail(token string) error {
	// Find verification record
	var verification models.EmailVerification
	if err := s.db.Where("token = ?", token).First(&verification).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("invalid verification token")
		}
		return err
	}

	// Check if token is already used
	if verification.IsUsed {
		return errors.New("verification token already used")
	}

	// Check if token has expired
	if time.Now().After(verification.ExpiresAt) {
		return errors.New("verification token has expired")
	}

	// Mark token as used
	if err := s.db.Model(&verification).Update("is_used", true).Error; err != nil {
		return err
	}

	// Mark restaurant email as verified
	if err := s.db.Model(&models.Restaurant{}).Where("id = ?", verification.RestaurantID).Update("is_email_verified", true).Error; err != nil {
		return err
	}

	return nil
}

// GetEmailVerificationStatus returns whether the restaurant email matches and is verified.
func (s *AuthService) GetEmailVerificationStatus(restaurantID, email string) (bool, error) {
	restaurantID = strings.TrimSpace(restaurantID)
	email = strings.TrimSpace(email)
	if restaurantID == "" || email == "" {
		return false, errors.New("restaurant_id and email are required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ? AND email = ?", restaurantID, email).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, errors.New("restaurant not found for that email")
		}
		return false, err
	}

	return restaurant.IsEmailVerified, nil
}

// ResendVerificationEmail resends verification email only when restaurant_id + email match.
// Does not return the verification link to the client.
func (s *AuthService) ResendVerificationEmail(restaurantID, email string) error {
	restaurantID = strings.TrimSpace(restaurantID)
	email = strings.TrimSpace(email)
	if restaurantID == "" || email == "" {
		return errors.New("restaurant_id and email are required")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ? AND email = ?", restaurantID, email).First(&restaurant).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("restaurant not found for that email")
		}
		return err
	}

	// Invalidate old tokens
	s.db.Model(&models.EmailVerification{}).Where("restaurant_id = ? AND email = ? AND is_used = false", restaurantID, email).Update("is_used", true)

	if _, err := s.SendVerificationEmail(restaurantID, email); err != nil {
		return err
	}
	return nil
}

// Login authenticates user and returns tokens.
// Accepts 8-digit admin login number (100xxxxx), 6-digit staff key, or legacy SK_ key.
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	var user models.User
	var err error

	identifier := strings.TrimSpace(req.Identifier)

	switch {
	case loginAdminKeyPattern.MatchString(identifier):
		err = s.db.Where("staff_key = ?", identifier).First(&user).Error
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid login number or password")
		}
	case strings.HasPrefix(identifier, "SK_") || loginStaffKeyPattern.MatchString(identifier):
		err = s.db.Where("staff_key = ?", identifier).First(&user).Error
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid login number or password")
		}
	default:
		return nil, errors.New("invalid login number format. Use your 8-digit admin number or 6-digit staff key")
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", user.RestaurantID).First(&restaurant).Error; err != nil {
		return nil, errors.New("restaurant not found")
	}
	if !restaurant.IsActive {
		return nil, errors.New("restaurant account is suspended. Contact BillGenie support")
	}
	// Holiday / closed mode: only the owner (admin) may sign in and operate.
	if restaurant.IsClosed && user.Role != "admin" {
		return nil, errors.New("restaurant is closed. Contact the owner to reopen")
	}
	if !restaurant.IsEmailVerified {
		var verificationCount int64
		if err := s.db.Model(&models.EmailVerification{}).Where("restaurant_id = ?", restaurant.ID).Count(&verificationCount).Error; err != nil {
			return nil, err
		}
		if verificationCount > 0 {
			return nil, errors.New("please verify your email before signing in. Check your inbox for the verification link sent during registration")
		}
	}
	if !restaurant.IsApproved {
		return nil, errors.New("your email is verified. Your restaurant is pending BillGenie approval — we'll email you as soon as you're approved and can sign in")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid password")
	}

	// Single active session per user (all roles): revoke previous sessions and refresh tokens.
	if err := s.RevokeAllSessionsForUser(user.ID); err != nil {
		return nil, fmt.Errorf("failed to revoke previous sessions: %w", err)
	}

	// Generate tokens
	accessToken, err := s.GenerateAccessToken(&user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.GenerateRefreshToken(&user)
	if err != nil {
		return nil, err
	}

	if err := s.CreateUserSession(&user, accessToken); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &AuthResponse{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       3600, // 1 hour for access token
		TokenType:       "Bearer",
		RestaurantID:    user.RestaurantID,
		UserID:          user.ID,
		Role:            user.Role,
		Name:            user.Name,
		CanCancelOrders:      user.CanCancelOrders,
		CanRestockInventory:  user.CanRestockInventory,
		MenuManagementAccess: user.MenuManagementAccess,
	}, nil
}

// GenerateAccessToken creates a new JWT access token
func (s *AuthService) GenerateAccessToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour)

	claims := &TokenClaims{
		UserID:       user.ID,
		RestaurantID: user.RestaurantID,
		Role:         user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// GenerateRefreshToken creates a refresh token valid for 7 days and stores in DB
func (s *AuthService) GenerateRefreshToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(7 * 24 * time.Hour)

	claims := &TokenClaims{
		UserID:       user.ID,
		RestaurantID: user.RestaurantID,
		Role:         user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", err
	}

	// Store in database
	refreshToken := &models.RefreshToken{
		UserID:    user.ID,
		Token:     tokenString,
		ExpiresAt: expirationTime,
	}

	if err := s.db.Create(refreshToken).Error; err != nil {
		return "", err
	}

	return tokenString, nil
}

// RefreshAccessToken validates refresh token and returns new access token
func (s *AuthService) RefreshAccessToken(refreshTokenStr string) (*AuthResponse, error) {
	// Validate refresh token
	claims, err := s.ValidateToken(refreshTokenStr)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Check if refresh token exists in database and is not expired
	var refreshToken models.RefreshToken
	if err := s.db.Where("token = ? AND expires_at > ?", refreshTokenStr, time.Now()).First(&refreshToken).Error; err != nil {
		return nil, errors.New("refresh token not found or expired")
	}

	// Get user
	var user models.User
	if err := s.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return nil, errors.New("user not found")
	}

	var activeSession models.UserSession
	if err := s.db.Where("user_id = ? AND is_active = true", user.ID).First(&activeSession).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("session invalidated. Another device has logged in with your account")
		}
		return nil, err
	}

	// Generate new access token
	newAccessToken, err := s.GenerateAccessToken(&user)
	if err != nil {
		return nil, err
	}

	// Keep the previous token valid briefly so concurrent in-flight requests
	// that still use the old JWT are not treated as a foreign-device login.
	graceUntil := time.Now().Add(60 * time.Second)
	if err := s.db.Model(&activeSession).Updates(map[string]interface{}{
		"previous_access_token":             activeSession.AccessToken,
		"previous_access_token_valid_until": graceUntil,
		"access_token":                      newAccessToken,
		"last_activity":                     time.Now(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return &AuthResponse{
		AccessToken:     newAccessToken,
		RefreshToken:    refreshTokenStr,
		ExpiresIn:       3600, // 1 hour
		TokenType:       "Bearer",
		RestaurantID:    user.RestaurantID,
		UserID:          user.ID,
		Role:            user.Role,
		Name:            user.Name,
		CanCancelOrders:      user.CanCancelOrders,
		CanRestockInventory:  user.CanRestockInventory,
		MenuManagementAccess: user.MenuManagementAccess,
	}, nil
}

// GetUserByID loads a user record for profile display.
func (s *AuthService) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// ValidateToken validates and parses JWT token
func (s *AuthService) ValidateToken(tokenString string) (*TokenClaims, error) {
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ValidateUserSession checks if a user has an active session matching the access token.
// After a refresh, the previous access token remains valid for a short grace window
// so in-flight requests are not mistaken for a login from another device.
func (s *AuthService) ValidateUserSession(userID string, accessToken string) (bool, error) {
	var session models.UserSession

	if err := s.db.Where("user_id = ? AND access_token = ? AND is_active = true", userID, accessToken).First(&session).Error; err == nil {
		return true, nil
	} else if err != gorm.ErrRecordNotFound {
		return false, err
	}

	now := time.Now()
	if err := s.db.Where(
		"user_id = ? AND previous_access_token = ? AND is_active = true AND previous_access_token_valid_until > ?",
		userID, accessToken, now,
	).First(&session).Error; err == nil {
		return true, nil
	} else if err != gorm.ErrRecordNotFound {
		return false, err
	}

	return false, errors.New("session invalidated. Another device has logged in with your account")
}

// RevokeAllSessionsForUser deactivates sessions and invalidates refresh tokens for a user.
func (s *AuthService) RevokeAllSessionsForUser(userID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.UserSession{}).
			Where("user_id = ? AND is_active = true", userID).
			Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.RefreshToken{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// RevokeRestaurantSessionsExceptCurrent kicks every active session in the restaurant
// except the caller's current access token. Returns user IDs that had sessions revoked.
func (s *AuthService) RevokeRestaurantSessionsExceptCurrent(
	restaurantID string,
	keepUserID string,
	keepAccessToken string,
) ([]string, error) {
	var sessions []models.UserSession
	if err := s.db.Where("restaurant_id = ? AND is_active = true", restaurantID).
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	revokedUsers := make(map[string]struct{})
	for _, session := range sessions {
		if session.UserID == keepUserID && session.AccessToken == keepAccessToken {
			continue
		}
		revokedUsers[session.UserID] = struct{}{}
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&models.UserSession{}).
			Where("restaurant_id = ? AND is_active = true", restaurantID)
		if keepAccessToken != "" {
			q = q.Where("access_token <> ?", keepAccessToken)
		}
		if err := q.Update("is_active", false).Error; err != nil {
			return err
		}

		// Drop refresh tokens for every user in the restaurant except keep-user (they re-auth next).
		var userIDs []string
		if err := tx.Model(&models.User{}).
			Where("restaurant_id = ?", restaurantID).
			Pluck("id", &userIDs).Error; err != nil {
			return err
		}
		for _, uid := range userIDs {
			if uid == keepUserID {
				// Still wipe other devices' refresh tokens for this admin by deleting all then...
				// Simpler: delete all refresh tokens for restaurant users; current access JWT still works until expiry.
				continue
			}
			if err := tx.Where("user_id = ?", uid).Delete(&models.RefreshToken{}).Error; err != nil {
				return err
			}
		}
		// Also clear refresh tokens for keep user so only the current access session remains usable for refresh.
		if err := tx.Where("user_id = ?", keepUserID).Delete(&models.RefreshToken{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(revokedUsers))
	for uid := range revokedUsers {
		out = append(out, uid)
	}
	return out, nil
}

// RevokeNonAdminSessionsForRestaurant kicks staff/manager/chef sessions (used when closing).
func (s *AuthService) RevokeNonAdminSessionsForRestaurant(restaurantID string) ([]string, error) {
	var users []models.User
	if err := s.db.Where("restaurant_id = ? AND role <> ?", restaurantID, "admin").
		Find(&users).Error; err != nil {
		return nil, err
	}
	revoked := make([]string, 0, len(users))
	for _, u := range users {
		if err := s.RevokeAllSessionsForUser(u.ID); err != nil {
			return revoked, err
		}
		revoked = append(revoked, u.ID)
	}
	return revoked, nil
}

// CreateUserSession records a new active session for the user.
func (s *AuthService) CreateUserSession(user *models.User, accessToken string) error {
	session := &models.UserSession{
		ID:                            uuid.New().String(),
		UserID:                        user.ID,
		RestaurantID:                  user.RestaurantID,
		AccessToken:                   accessToken,
		PreviousAccessToken:           "",
		PreviousAccessTokenValidUntil: nil,
		IsActive:                      true,
	}
	return s.db.Create(session).Error
}

// LogoutUser deactivates the current session and refresh tokens for the user.
func (s *AuthService) LogoutUser(userID string, accessToken string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.UserSession{}).
			Where("user_id = ? AND access_token = ? AND is_active = true", userID, accessToken).
			Update("is_active", false).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.RefreshToken{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// DeactivateUserSession logs out a user by deactivating all active sessions
func (s *AuthService) DeactivateUserSession(userID string) error {
	if err := s.db.Model(&models.UserSession{}).Where("user_id = ? AND is_active = true", userID).Update("is_active", false).Error; err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	return nil
}

// Helper functions

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// generateRestaurantCode creates a unique 6-character code from restaurant name
func generateRestaurantCode(restaurantName string) string {
	// Clean restaurant name: remove spaces, take first 4 chars, uppercase
	cleaned := strings.ToUpper(strings.ReplaceAll(restaurantName, " ", ""))
	if len(cleaned) > 4 {
		cleaned = cleaned[:4]
	}

	// Add 2 random digits
	randomSuffix := rand.Intn(100) // 00-99
	code := cleaned + fmt.Sprintf("%02d", randomSuffix)

	// Ensure minimum length of 6
	if len(code) < 6 {
		code = code + strings.Repeat("0", 6-len(code))
	}

	return code
}

func normalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

// generateStaffKey creates a globally unique staff key (e.g., SK_8F4K9P2Q1R)
func generateStaffKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := "SK_" // Staff Key prefix

	for i := 0; i < 10; i++ {
		key += string(charset[rand.Intn(len(charset))])
	}

	return key
}

// ForgotPassword generates a password reset token and emails the reset link.
// Lookup is by registered email or phone (admin accounts only).
func (s *AuthService) ForgotPassword(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", errors.New("email or phone number is required")
	}

	// Login numbers cannot be used for password recovery
	if loginAdminKeyPattern.MatchString(identifier) ||
		loginStaffKeyPattern.MatchString(identifier) ||
		strings.HasPrefix(identifier, "SK_") {
		return "", errors.New("use your registered email or phone number to reset your password")
	}

	var user models.User
	var err error

	if strings.Contains(identifier, "@") {
		err = s.db.Where("email = ?", identifier).First(&user).Error
	} else {
		normalized := normalizePhone(identifier)
		if len(normalized) < 10 {
			return "", errors.New("please enter a valid email or phone number")
		}
		err = s.db.Where(
			"regexp_replace(phone, '[^0-9]', '', 'g') = ? OR regexp_replace(phone, '[^0-9]', '', 'g') LIKE ?",
			normalized,
			"%"+normalized,
		).First(&user).Error
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", errors.New("no account found with that email or phone number")
		}
		return "", err
	}

	// Only admins can self-reset; staff should ask their admin
	if user.Role != "admin" {
		return "", errors.New("staff members cannot reset password here. Please ask your admin to reset your password")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", user.RestaurantID).First(&restaurant).Error; err != nil {
		return "", errors.New("restaurant not found")
	}
	if !restaurant.IsEmailVerified {
		return "", errors.New("your email is not verified yet. Check your inbox for the verification link sent during registration")
	}

	// Generate reset token (32-character random string)
	resetToken := generateRandomToken(32)

	// Create password reset record (valid for 1 hour)
	passwordReset := &models.PasswordReset{
		UserID:    user.ID,
		Email:     user.Email,
		Token:     resetToken,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		IsUsed:    false,
	}

	if err := s.db.Create(passwordReset).Error; err != nil {
		return "", fmt.Errorf("failed to create password reset token: %w", err)
	}

	publicBase := publicAppBaseURL()
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", publicBase, resetToken)

	subject := "Reset your BillGenie password"
	body := fmt.Sprintf(
		"Hi,\n\nYou requested to reset your password.\n\nOpen this link to choose a new password:\n%s\n\n"+
			"This link expires in 1 hour and works only once.\nIf you did not request this, ignore this email.\n\n- BillGenie",
		resetLink,
	)

	if err := sendEmailSMTP(user.Email, subject, body); err != nil {
		return "", fmt.Errorf("failed to send reset email: %w", err)
	}

	return resetLink, nil
}

// ResetPassword validates token and updates user password
func (s *AuthService) ResetPassword(token string, newPassword string) error {
	// Validate input
	if token == "" || newPassword == "" {
		return errors.New("token and password are required")
	}
	if err := ValidateAccountPassword(newPassword, ""); err != nil {
		return err
	}

	// Find valid, unused reset token
	var passwordReset models.PasswordReset
	if err := s.db.Where("token = ? AND is_used = false AND expires_at > ?", token, time.Now()).First(&passwordReset).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.New("invalid or expired reset token")
		}
		return err
	}

	// Hash new password
	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user password in a transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Update password
		if err := tx.Model(&models.User{}).Where("id = ?", passwordReset.UserID).Update("password_hash", hashedPassword).Error; err != nil {
			return err
		}

		// Mark token as used
		if err := tx.Model(&models.PasswordReset{}).Where("id = ?", passwordReset.ID).Update("is_used", true).Error; err != nil {
			return err
		}

		// Invalidate all existing sessions for this user (force re-login on all devices)
		if err := tx.Model(&models.UserSession{}).Where("user_id = ?", passwordReset.UserID).Update("is_active", false).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	return nil
}

const loginRecoveryOTPExpiry = 10 * time.Minute
const loginRecoveryMaxAttempts = 5

// RequestLoginRecovery emails a 6-digit OTP so an admin can recover their login number.
func (s *AuthService) RequestLoginRecovery(identifier string) error {
	user, err := s.findAdminByRecoveryIdentifier(identifier)
	if err != nil {
		return err
	}

	if strings.TrimSpace(user.Email) == "" {
		return errors.New("no email on file for this account. Contact support")
	}

	otp := generateNumericOTP(6)

	if err := s.db.Model(&models.LoginRecoveryOTP{}).
		Where("user_id = ? AND is_used = false", user.ID).
		Update("is_used", true).Error; err != nil {
		return fmt.Errorf("failed to invalidate previous recovery codes: %w", err)
	}

	recovery := &models.LoginRecoveryOTP{
		UserID:     user.ID,
		Identifier: strings.TrimSpace(identifier),
		OTP:        otp,
		ExpiresAt:  time.Now().Add(loginRecoveryOTPExpiry),
		IsUsed:     false,
		Attempts:   0,
	}

	if err := s.db.Create(recovery).Error; err != nil {
		return fmt.Errorf("failed to create recovery code: %w", err)
	}

	subject := "Your BillGenie login recovery code"
	body := fmt.Sprintf(
		"Hi,\n\nYour login recovery code is: %s\n\nThis code expires in 10 minutes.\nIf you did not request this, ignore this email.\n\n- BillGenie",
		otp,
	)

	if err := sendEmailSMTP(user.Email, subject, body); err != nil {
		return fmt.Errorf("failed to send recovery email: %w", err)
	}

	return nil
}

// VerifyLoginRecovery validates the OTP and returns the admin login number.
func (s *AuthService) VerifyLoginRecovery(identifier, otp string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	otp = strings.TrimSpace(otp)

	if identifier == "" || otp == "" {
		return "", errors.New("identifier and verification code are required")
	}
	if len(otp) != 6 {
		return "", errors.New("verification code must be 6 digits")
	}

	user, err := s.findAdminByRecoveryIdentifier(identifier)
	if err != nil {
		return "", err
	}

	var recovery models.LoginRecoveryOTP
	if err := s.db.Where(
		"user_id = ? AND is_used = false AND expires_at > ?",
		user.ID,
		time.Now(),
	).Order("created_at DESC").First(&recovery).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", errors.New("no active recovery code. Request a new one")
		}
		return "", err
	}

	if recovery.Attempts >= loginRecoveryMaxAttempts {
		return "", errors.New("too many failed attempts. Request a new code")
	}

	if recovery.OTP != otp {
		s.db.Model(&recovery).Update("attempts", recovery.Attempts+1)
		return "", errors.New("invalid verification code")
	}

	if err := s.db.Model(&recovery).Update("is_used", true).Error; err != nil {
		return "", fmt.Errorf("failed to mark recovery code used: %w", err)
	}

	return user.StaffKey, nil
}

func (s *AuthService) findAdminByRecoveryIdentifier(identifier string) (*models.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, errors.New("email or phone number is required")
	}

	if loginAdminKeyPattern.MatchString(identifier) ||
		loginStaffKeyPattern.MatchString(identifier) ||
		strings.HasPrefix(identifier, "SK_") {
		return nil, errors.New("use your registered email or phone number to recover your login number")
	}

	var user models.User
	var err error

	if strings.Contains(identifier, "@") {
		err = s.db.Where("email = ?", identifier).First(&user).Error
	} else {
		normalized := normalizePhone(identifier)
		if len(normalized) < 10 {
			return nil, errors.New("please enter a valid email or phone number")
		}
		err = s.db.Where(
			"regexp_replace(phone, '[^0-9]', '', 'g') = ? OR regexp_replace(phone, '[^0-9]', '', 'g') LIKE ?",
			normalized,
			"%"+normalized,
		).First(&user).Error
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("no account found with that email or phone number")
		}
		return nil, err
	}

	if user.Role != "admin" {
		return nil, errors.New("login recovery is only available for restaurant admin accounts")
	}

	var restaurant models.Restaurant
	if err := s.db.Where("id = ?", user.RestaurantID).First(&restaurant).Error; err != nil {
		return nil, errors.New("restaurant not found")
	}
	if !restaurant.IsEmailVerified {
		return nil, errors.New("your email is not verified yet. Check your inbox for the verification link sent during registration")
	}

	return &user, nil
}

func generateNumericOTP(length int) string {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return string(b)
}

func generateRandomToken(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// publicAppBaseURL is the HTTPS base for links in emails (reset password, verify email).
// Falls back to API_BASE_URL so Fly deployments work without a separate PUBLIC_APP_URL.
func publicAppBaseURL() string {
	if base := strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")); base != "" {
		return strings.TrimRight(base, "/")
	}
	if base := strings.TrimSpace(os.Getenv("API_BASE_URL")); base != "" {
		return strings.TrimRight(base, "/")
	}
	return "http://localhost:3000"
}

// sendEmailSMTP sends an email using SMTP settings from environment variables
// Required env vars: SMTP_HOST, SMTP_PORT and either
// SMTP_USER/SMTP_PASS or SMTP_MAIL/SMTP_APP_PASSWORD
// Optional: SMTP_FROM (defaults to SMTP_USER/SMTP_MAIL)
func sendEmailSMTP(to, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	if user == "" {
		user = os.Getenv("SMTP_MAIL")
	}
	pass := os.Getenv("SMTP_PASS")
	if pass == "" {
		pass = os.Getenv("SMTP_APP_PASSWORD")
	}
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}

	if host == "" || port == "" || user == "" || pass == "" {
		return errors.New("smtp is not configured (SMTP_HOST/SMTP_PORT and SMTP_USER/SMTP_PASS or SMTP_MAIL/SMTP_APP_PASSWORD)")
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	auth := smtp.PlainAuth("", user, pass, host)

	msg := []byte(
		"To: " + to + "\r\n" +
			"From: " + from + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
			"\r\n" +
			body,
	)

	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}
