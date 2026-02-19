package services

import (
	"errors"
	"fmt"
	"math/rand"
	"net/smtp"
	"os"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
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
	Role         string `json:"role"`
}

type RegisterRequest struct {
	RestaurantName string `json:"restaurant_name" validate:"required"`
	OwnerName      string `json:"owner_name" validate:"required"`
	Email          string `json:"email" validate:"required,email"`
	Phone          string `json:"phone" validate:"required"`
	Password       string `json:"password" validate:"required,min=6"`
	Address        string `json:"address"`
	City           string `json:"city"`
	Cuisine        string `json:"cuisine"`
}

type LoginRequest struct {
	Identifier string `json:"identifier" validate:"required"` // Email (admin) or Staff Key (staff)
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

	// Create restaurant with 30-day free trial
	restaurant := &models.Restaurant{
		ID:              uuid.New().String(),
		RestaurantCode:  restaurantCode,
		Name:            req.RestaurantName,
		OwnerName:       req.OwnerName,
		Email:           req.Email,
		Phone:           req.Phone,
		Address:         req.Address,
		City:            req.City,
		Cuisine:         req.Cuisine,
		IsActive:        true,
		SubscriptionEnd: time.Now().AddDate(0, 0, 30), // 30-day free trial
	}

	if err := s.db.Create(restaurant).Error; err != nil {
		return nil, nil, err
	}

	// Create admin user with auto-generated staff key
	staffKey := generateStaffKey()
	user := &models.User{
		ID:           uuid.New().String(),
		RestaurantID: restaurant.ID,
		Name:         req.OwnerName,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hashedPassword,
		Role:         "admin",
		IsActive:     true,
		StaffKey:     staffKey,
	}

	if err := s.db.Create(user).Error; err != nil {
		// Rollback restaurant creation if user creation fails
		s.db.Delete(restaurant)
		return nil, nil, err
	}

	// Send verification email
	if _, err := s.SendVerificationEmail(restaurant.ID, req.Email); err != nil {
		fmt.Printf("⚠️  Warning: Failed to send verification email: %v\n", err)
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

	publicBase := os.Getenv("PUBLIC_APP_URL")
	if publicBase == "" {
		publicBase = "http://localhost:3000"
	}
	verificationLink := fmt.Sprintf("%s/verify-email?token=%s", publicBase, token)
	deepLink := fmt.Sprintf("billgenie://verify-email?token=%s", token)

	subject := "Verify your BillGenie email"
	body := fmt.Sprintf("Hi,\n\nPlease verify your email by clicking this link:\n%s\n\nIf the link does not open the app, you can use this app link:\n%s\n\nThis link expires in 24 hours.\n\n- BillGenie", verificationLink, deepLink)

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

// ResendVerificationEmail resends verification email
func (s *AuthService) ResendVerificationEmail(restaurantID, email string) (string, error) {
	// Invalidate old tokens
	s.db.Model(&models.EmailVerification{}).Where("restaurant_id = ? AND email = ? AND is_used = false", restaurantID, email).Update("is_used", true)

	// Send new verification email
	return s.SendVerificationEmail(restaurantID, email)
}

// Login authenticates user and returns tokens
// Accepts either email (admin) or staff_key (staff) as identifier
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	var user models.User
	var err error

	// Determine if identifier is email or staff key
	if strings.Contains(req.Identifier, "@") {
		// Email-based login (admin)
		err = s.db.Where("email = ?", req.Identifier).First(&user).Error
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid email or password")
		}
	} else if strings.HasPrefix(req.Identifier, "SK_") {
		// Staff key-based login (staff/manager/chef)
		err = s.db.Where("staff_key = ?", req.Identifier).First(&user).Error
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid staff key or password")
		}
	} else {
		return nil, errors.New("invalid identifier format. Please enter email or staff key")
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid password")
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

	// Create session record (invalidate previous sessions for this user)
	// Only allow 1 active session per user (for staff/chef accounts)
	// Admin accounts can have multiple sessions
	if user.Role != "admin" {
		// Deactivate all previous sessions for this user
		if err := s.db.Model(&models.UserSession{}).Where("user_id = ? AND is_active = true", user.ID).Update("is_active", false).Error; err != nil {
			fmt.Printf("⚠️  Warning: Failed to deactivate previous sessions: %v\n", err)
		}
	}

	// Create new session
	session := &models.UserSession{
		ID:           uuid.New().String(),
		UserID:       user.ID,
		RestaurantID: user.RestaurantID,
		AccessToken:  accessToken,
		IsActive:     true,
	}

	if err := s.db.Create(session).Error; err != nil {
		fmt.Printf("⚠️  Warning: Failed to create session record: %v\n", err)
		// Continue anyway - session tracking is non-critical
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600, // 1 hour for access token
		TokenType:    "Bearer",
		RestaurantID: user.RestaurantID,
		UserID:       user.ID,
		Role:         user.Role,
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

	// Generate new access token
	newAccessToken, err := s.GenerateAccessToken(&user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: refreshTokenStr,
		ExpiresIn:    3600, // 1 hour
		TokenType:    "Bearer",
		RestaurantID: user.RestaurantID,
		UserID:       user.ID,
		Role:         user.Role,
	}, nil
}

// ValidateToken validates and parses JWT token
func (s *AuthService) ValidateToken(tokenString string) (*TokenClaims, error) {
	claims := &TokenClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
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

// ValidateUserSession checks if a user has an active session
// Used to enforce single concurrent login for staff/chef accounts
func (s *AuthService) ValidateUserSession(userID string, accessToken string) (bool, error) {
	var session models.UserSession

	if err := s.db.Where("user_id = ? AND access_token = ? AND is_active = true", userID, accessToken).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, errors.New("session not found or expired. Please login again")
		}
		return false, err
	}

	return true, nil
}

// DeactivateUserSession logs out a user by deactivating their session
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

// generateStaffKey creates a globally unique staff key (e.g., SK_8F4K9P2Q1R)
func generateStaffKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key := "SK_" // Staff Key prefix

	for i := 0; i < 10; i++ {
		key += string(charset[rand.Intn(len(charset))])
	}

	return key
}

// ForgotPassword generates a password reset token and returns reset link
// Note: Only admin users can reset password via email
// Staff users should ask admin to regenerate their staff key instead
func (s *AuthService) ForgotPassword(identifier string) (string, error) {
	// Find user by email or staff key
	var user models.User

	if strings.Contains(identifier, "@") {
		// Email login (admin)
		if err := s.db.Where("email = ?", identifier).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return "", errors.New("user not found")
			}
			return "", err
		}
	} else if strings.HasPrefix(identifier, "SK_") {
		// Staff key login (staff) - not allowed to reset password via email
		return "", errors.New("staff members cannot reset password via email. Please ask your admin to regenerate your staff key")
	} else {
		return "", errors.New("invalid identifier format")
	}

	// Check if user is admin (only admins can reset password)
	if user.Role != "admin" {
		return "", errors.New("only admin users can reset password via email. Staff members should ask their admin to regenerate their staff key")
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

	publicBase := os.Getenv("PUBLIC_APP_URL")
	if publicBase == "" {
		publicBase = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", publicBase, resetToken)
	deepLink := fmt.Sprintf("billgenie://reset-password?token=%s", resetToken)

	subject := "Reset your BillGenie password"
	body := fmt.Sprintf("Hi,\n\nYou requested to reset your password. Click the link below to set a new password:\n%s\n\nIf the link does not open the app, you can use this app link:\n%s\n\nThis link expires in 1 hour.\nIf you did not request this, ignore this email.\n\n- BillGenie", resetLink, deepLink)

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

// generateRandomToken generates a random token of specified length
func generateRandomToken(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// sendEmailSMTP sends an email using SMTP settings from environment variables
// Required env vars: SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS
// Optional: SMTP_FROM (defaults to SMTP_USER)
func sendEmailSMTP(to, subject, body string) error {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}

	if host == "" || port == "" || user == "" || pass == "" {
		return errors.New("smtp is not configured (SMTP_HOST/SMTP_PORT/SMTP_USER/SMTP_PASS)")
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
