package services

import (
	"errors"
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
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
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

	// Create restaurant
	restaurant := &models.Restaurant{
		ID:        uuid.New().String(),
		Name:      req.RestaurantName,
		OwnerName: req.OwnerName,
		Email:     req.Email,
		Phone:     req.Phone,
		Address:   req.Address,
		City:      req.City,
		Cuisine:   req.Cuisine,
		IsActive:  true,
	}

	if err := s.db.Create(restaurant).Error; err != nil {
		return nil, nil, err
	}

	// Create admin user
	user := &models.User{
		ID:           uuid.New().String(),
		RestaurantID: restaurant.ID,
		Name:         req.OwnerName,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: hashedPassword,
		Role:         "admin",
		IsActive:     true,
	}

	if err := s.db.Create(user).Error; err != nil {
		// Rollback restaurant creation if user creation fails
		s.db.Delete(restaurant)
		return nil, nil, err
	}

	return restaurant, user, nil
}

// Login authenticates user and returns tokens
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	// Find user by email
	var user models.User
	if err := s.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Generate tokens
	accessToken, err := s.GenerateAccessToken(&user)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		AccessToken: accessToken,
		ExpiresIn:   3600, // 1 hour
		TokenType:   "Bearer",
	}, nil
}

// GenerateAccessToken creates a new JWT access token
func (s *AuthService) GenerateAccessToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(time.Hour)

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

// Helper functions

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}
