package handlers

import (
	"fmt"
	"log"
	"net/http"

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

	// Generate JWT tokens for the newly created admin user
	accessToken, err := h.authService.GenerateAccessToken(user)
	if err != nil {
		log.Printf("❌ Access token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate access token"})
		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(user)
	if err != nil {
		log.Printf("❌ Refresh token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	log.Printf("✅ JWT tokens generated for user: %s", user.ID)

	c.JSON(http.StatusCreated, gin.H{
		"access_token":    accessToken,
		"refresh_token":   refreshToken,
		"expires_in":      3600,
		"token_type":      "Bearer",
		"restaurant_id":   restaurant.ID,
		"restaurant_code": restaurant.RestaurantCode,
		"user_id":         user.ID,
		"role":            user.Role,
		"message":         fmt.Sprintf("Restaurant registered successfully! Your login code is: %s", restaurant.RestaurantCode),
	})
}

// Login handles user login
// @Summary User login
// @Description Login with email and password to get JWT token
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

	c.JSON(http.StatusOK, authResponse)
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

	log.Printf("✅ Profile retrieved for user: %s", userID)

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"restaurant_id": restaurantID,
		"role":          role,
		"message":       "Profile retrieved successfully",
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
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Binding error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Refresh the access token
	authResponse, err := h.authService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		log.Printf("❌ Token refresh failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Access token refreshed successfully")

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
// @Param request body map[string]string true "User identifier (email or staff key)"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Password reset token generated for identifier: %s", req.Identifier)
	log.Printf("🔗 Reset link emailed to user")

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset link has been sent to your email",
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
		NewPassword string `json:"new_password" validate:"required,min=6"`
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
		"message": "Email verified successfully. You can now login.",
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

	verificationLink, err := h.authService.ResendVerificationEmail(restaurantID, email)
	if err != nil {
		log.Printf("❌ Resend verification email error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resend verification email"})
		return
	}

	log.Printf("✅ Verification email resent to %s", email)

	c.JSON(http.StatusOK, gin.H{
		"message": "Verification email resent successfully",
		"link":    verificationLink, // For testing only, remove in production
	})
}
