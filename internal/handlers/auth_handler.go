package handlers

import (
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

	log.Printf("✅ New restaurant registered: %s (ID: %s)", restaurant.Name, restaurant.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful",
		"restaurant": gin.H{
			"id":    restaurant.ID,
			"name":  restaurant.Name,
			"email": restaurant.Email,
		},
		"admin_user": gin.H{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"role":  user.Role,
		},
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
