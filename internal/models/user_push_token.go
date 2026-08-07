package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserPushToken stores an Expo push token for background staff alerts.
type UserPushToken struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	UserID       string    `json:"user_id" gorm:"index;not null"`
	RestaurantID string    `json:"restaurant_id" gorm:"index;not null"`
	Token        string    `json:"token" gorm:"type:varchar(255);uniqueIndex;not null"`
	Platform     string    `json:"platform" gorm:"type:varchar(16)"` // ios | android
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (UserPushToken) TableName() string {
	return "user_push_tokens"
}

func (t *UserPushToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}
