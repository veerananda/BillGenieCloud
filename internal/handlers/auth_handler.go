package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type AuthHandler struct {
	authService *services.AuthService
	validator   *validator.Validate
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validator:   validator.New(),
	}
}

// Register handles user registration
// @Summary Register a new restaurant
// @Description Create a new restaurant account with admin user
// @Accept json
// @Produce json
// @Param request body services.RegisterRequest true "Registration data"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req services.RegisterRequest

	// Bind JSON request body
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

	// Register restaurant and user
	restaurant, user, err := h.authService.Register(req)
	if err != nil {
		log.Printf("❌ Registration error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ New restaurant registered: %s (ID: %s, Code: %s)", restaurant.Name, restaurant.ID, restaurant.RestaurantCode)

	emailSent := true
	if _, err := h.authService.SendVerificationEmail(restaurant.ID, restaurant.Email); err != nil {
		log.Printf("⚠️ Verification email failed for %s: %v", restaurant.Email, err)
		emailSent = false
	}

	c.JSON(http.StatusCreated, gin.H{
		"restaurant_id":               restaurant.ID,
		"restaurant_code":             restaurant.RestaurantCode,
		"email":                       restaurant.Email,
		"user_id":                     user.ID,
		"role":                        user.Role,
		"login_id":                    user.StaffKey,
		"staff_key":                   user.StaffKey,
		"subscription_phase":          services.ParseStoredSubscriptionConfig(restaurant).Phase,
		"requires_payment":            services.ParseStoredSubscriptionConfig(restaurant).Phase == services.SubscriptionPhasePendingPayment,
		"requires_email_verification": true,
		"is_email_verified":           restaurant.IsEmailVerified,
		"requires_approval":           true,
		"is_approved":                 restaurant.IsApproved,
		"verification_email_sent":     emailSent,
		"message":                     fmt.Sprintf("Restaurant registered successfully! Verify your email, then wait for BillGenie approval before signing in with login number: %s", user.StaffKey),
	})
}

// Login handles user login
// @Summary User login
// @Description Login with login number and password to get JWT token
// @Accept json
// @Produce json
// @Param request body services.LoginRequest true "Login credentials"
// @Success 200 {object} services.AuthResponse
// @Failure 401 {object} map[string]interface{}
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req services.LoginRequest

	// Bind JSON request body
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

	// Authenticate user
	authResponse, err := h.authService.Login(req)
	if err != nil {
		log.Printf("❌ Login failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ User logged in successfully")

	BroadcastSessionRevoked(authResponse.RestaurantID, authResponse.UserID)

	setRefreshTokenCookie(c, authResponse.RefreshToken, int((7 * 24 * time.Hour).Seconds()))
	c.JSON(http.StatusOK, authResponse)
}

// Logout deactivates the current session.
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, _ := c.Get("user_id")
	authHeader := c.GetHeader("Authorization")
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authorization header"})
		return
	}

	if err := h.authService.LogoutUser(userID.(string), parts[1]); err != nil {
		log.Printf("❌ Logout failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}

	clearRefreshTokenCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// GetProfile retrieves current user profile
// @Summary Get user profile
// @Description Get profile of authenticated user
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	restaurantID, _ := c.Get("restaurant_id")
	role, _ := c.Get("role")

	user, err := h.authService.GetUserByID(userID.(string))
	if err != nil {
		log.Printf("❌ Profile lookup failed: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	log.Printf("✅ Profile retrieved for user: %s", userID)

	c.JSON(http.StatusOK, gin.H{
		"user_id":           userID,
		"restaurant_id":     restaurantID,
		"role":              role,
		"name":              user.Name,
		"can_cancel_orders":       user.CanCancelOrders,
		"can_restock_inventory":  user.CanRestockInventory,
		"menu_management_access": user.MenuManagementAccess,
		"message":                "Profile retrieved successfully",
	})
}

// RefreshToken refreshes access token using refresh token
// @Summary Refresh access token
// @Description Get new access token using refresh token
// @Accept json
// @Produce json
// @Param request body map[string]string true "Refresh token"
// @Success 200 {object} services.AuthResponse
// @Failure 401 {object} map[string]interface{}
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	// Body is optional when the httpOnly refresh cookie is present (web).
	_ = c.ShouldBindJSON(&req)

	refreshToken := readRefreshToken(c, req.RefreshToken)
	if refreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	authResponse, err := h.authService.RefreshAccessToken(refreshToken)
	if err != nil {
		log.Printf("❌ Token refresh failed: %v", err)
		clearRefreshTokenCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Access token refreshed successfully")

	setRefreshTokenCookie(c, authResponse.RefreshToken, int((7 * 24 * time.Hour).Seconds()))
	c.JSON(http.StatusOK, authResponse)
}

// HealthCheck is a simple health endpoint
// @Summary Health check
// @Description Returns 200 if server is healthy
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
func (h *AuthHandler) HealthCheck(c *gin.Context) {
	log.Println("✅ Health check endpoint called")
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"message": "Server is running",
	})
}

// ForgotPassword handles password reset requests
// @Summary Request password reset
// @Description Generate a password reset token and return reset link
// @Accept json
// @Produce json
// @Param request body map[string]string true "Registered email or phone number"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Identifier string `json:"identifier" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		log.Printf("❌ Validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := h.authService.ForgotPassword(req.Identifier)
	if err != nil {
		log.Printf("❌ Forgot password error: %v", err)
		if isAuthIdentifierFormatError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Avoid account enumeration: always return the same success message.
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If an account exists for that email or phone, a password reset link has been sent.",
	})
}

// ResetPassword handles password reset with valid token
// @Summary Reset password
// @Description Reset user password with valid reset token
// @Accept json
// @Produce json
// @Param request body map[string]string true "Reset token and new password"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		log.Printf("❌ Validation error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.ResetPassword(req.Token, req.NewPassword); err != nil {
		log.Printf("❌ Reset password error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Password reset successfully")

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset successfully. Please login with your new password.",
	})
}

// ForgotLoginID emails a one-time code to recover the admin login number.
// @Router /auth/forgot-login-id [post]
func (h *AuthHandler) ForgotLoginID(c *gin.Context) {
	var req struct {
		Identifier string `json:"identifier" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.RequestLoginRecovery(req.Identifier); err != nil {
		log.Printf("❌ Forgot login ID error: %v", err)
		if isAuthIdentifierFormatError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Avoid account enumeration.
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "If an account exists for that email or phone, a verification code has been sent.",
	})
}

// VerifyLoginRecovery validates the OTP and returns the admin login number.
// @Router /auth/verify-login-recovery [post]
func (h *AuthHandler) VerifyLoginRecovery(c *gin.Context) {
	var req struct {
		Identifier string `json:"identifier" validate:"required"`
		OTP        string `json:"otp" validate:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.validator.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loginID, err := h.authService.VerifyLoginRecovery(req.Identifier, req.OTP)
	if err != nil {
		log.Printf("❌ Verify login recovery error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"login_id": loginID,
		"message":  "Login number recovered successfully",
	})
}

// VerifyEmail verifies email with verification token
// @Summary Verify email address
// @Description Verify restaurant email with verification token
// @Accept json
// @Produce json
// @Param token query string true "Verification token"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token is required"})
		return
	}

	if err := h.authService.VerifyEmail(token); err != nil {
		log.Printf("❌ Email verification error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Email verified successfully")

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully. You can now sign in to BillGenie.",
	})
}

// GetVerificationStatus reports whether a restaurant email has been verified.
// @Router /auth/verification-status [get]
func (h *AuthHandler) GetVerificationStatus(c *gin.Context) {
	restaurantID := strings.TrimSpace(c.Query("restaurant_id"))
	email := strings.TrimSpace(c.Query("email"))

	if restaurantID == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "restaurant_id and email are required"})
		return
	}

	verified, err := h.authService.GetEmailVerificationStatus(restaurantID, email)
	if err != nil {
		log.Printf("❌ Verification status error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"is_email_verified": verified,
	})
}

// ResendVerificationEmail resends verification email
// @Summary Resend verification email
// @Description Resend verification email to restaurant email address
// @Accept json
// @Produce json
// @Param restaurantID query string true "Restaurant ID"
// @Param email query string true "Email address"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /auth/resend-verification [post]
func (h *AuthHandler) ResendVerificationEmail(c *gin.Context) {
	restaurantID := c.Query("restaurant_id")
	email := c.Query("email")

	if restaurantID == "" || email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Restaurant ID and email are required"})
		return
	}

	if err := h.authService.ResendVerificationEmail(restaurantID, email); err != nil {
		log.Printf("❌ Resend verification email error: %v", err)
		// Avoid account enumeration: same message whether restaurant/email match or not.
		c.JSON(http.StatusOK, gin.H{
			"message": "If that restaurant email is registered, a verification link has been sent",
		})
		return
	}

	log.Printf("✅ Verification email resent to %s", email)

	c.JSON(http.StatusOK, gin.H{
		"message": "If that restaurant email is registered, a verification link has been sent",
	})
}

// isAuthIdentifierFormatError reports client input/format mistakes that are safe to return
// verbatim (not account-existence signals).
func isAuthIdentifierFormatError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "required"):
		return true
	case strings.Contains(msg, "use your registered email"):
		return true
	case strings.Contains(msg, "valid email or phone"):
		return true
	case strings.Contains(msg, "login number"):
		return true
	default:
		return false
	}
}
