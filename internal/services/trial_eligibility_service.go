package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TrialEligibilityService struct {
	db *gorm.DB
}

func NewTrialEligibilityService(db *gorm.DB) *TrialEligibilityService {
	return &TrialEligibilityService{db: db}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizePhone(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	normalized := digits.String()
	if len(normalized) > 10 {
		normalized = normalized[len(normalized)-10:]
	}
	return normalized
}

func (s *TrialEligibilityService) EnsureTrialAvailable(email, phone string) error {
	emailNorm := NormalizeEmail(email)
	phoneNorm := NormalizePhone(phone)
	if emailNorm == "" || phoneNorm == "" {
		return errors.New("valid email and phone are required for a free trial")
	}

	var byEmail models.TrialEligibility
	if err := s.db.Where("email_normalized = ?", emailNorm).First(&byEmail).Error; err == nil {
		return errors.New("free trial already used for this email — choose Subscribe now or sign in to your existing account")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	var byPhone models.TrialEligibility
	if err := s.db.Where("phone_normalized = ?", phoneNorm).First(&byPhone).Error; err == nil {
		return errors.New("free trial already used for this phone number — choose Subscribe now or sign in to your existing account")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	return nil
}

func (s *TrialEligibilityService) RecordTrialGrant(restaurantID, email, phone string, expiresAt time.Time) error {
	record := models.TrialEligibility{
		ID:              uuid.New().String(),
		EmailNormalized: NormalizeEmail(email),
		PhoneNormalized: NormalizePhone(phone),
		RestaurantID:    restaurantID,
		GrantedAt:       time.Now(),
		ExpiresAt:       expiresAt,
	}
	return s.db.Create(&record).Error
}

func (s *TrialEligibilityService) MarkConverted(restaurantID string) error {
	now := time.Now()
	return s.db.Model(&models.TrialEligibility{}).
		Where("restaurant_id = ?", restaurantID).
		Update("converted_at", now).Error
}

func (s *TrialEligibilityService) CheckPhoneUniqueForRegister(phone string) error {
	phoneNorm := NormalizePhone(phone)
	if phoneNorm == "" {
		return errors.New("phone number is required")
	}
	var existing models.TrialEligibility
	if err := s.db.Where("phone_normalized = ?", phoneNorm).First(&existing).Error; err == nil {
		return fmt.Errorf("phone number already registered — sign in or choose Subscribe now without a trial")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}
