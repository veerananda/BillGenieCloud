package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PushAlertAssistance    = "assistance"
	PushAlertItemsReady    = "items_ready"
	PushAlertItemCancelled = "item_cancelled"
)

// PushService delivers Expo push notifications to staff devices.
type PushService struct {
	db     *gorm.DB
	client *http.Client
}

func NewPushService(db *gorm.DB) *PushService {
	return &PushService{
		db: db,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

type UpsertPushTokenInput struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform"`
}

func (s *PushService) UpsertToken(userID, restaurantID string, input UpsertPushTokenInput) error {
	token := strings.TrimSpace(input.Token)
	if token == "" {
		return fmt.Errorf("token required")
	}
	platform := strings.ToLower(strings.TrimSpace(input.Platform))
	if platform != "ios" && platform != "android" {
		platform = ""
	}
	row := models.UserPushToken{
		UserID:       userID,
		RestaurantID: restaurantID,
		Token:        token,
		Platform:     platform,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "restaurant_id", "platform", "updated_at"}),
	}).Create(&row).Error
}

func (s *PushService) DeleteToken(userID, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return s.db.Where("user_id = ? AND token = ?", userID, token).Delete(&models.UserPushToken{}).Error
}

func (s *PushService) DeleteAllForUser(userID string) error {
	return s.db.Where("user_id = ?", userID).Delete(&models.UserPushToken{}).Error
}

type expoPushMessage struct {
	To        string            `json:"to"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Sound     string            `json:"sound"`
	ChannelID string            `json:"channelId,omitempty"`
	Priority  string            `json:"priority"`
	Data      map[string]string `json:"data,omitempty"`
}

// NotifyRestaurantStaff sends a background-capable Expo push to all tokens for a restaurant.
func (s *PushService) NotifyRestaurantStaff(restaurantID, alertType, title, body string, data map[string]string) {
	if s == nil || strings.TrimSpace(restaurantID) == "" {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("push notify panic: %v", r)
			}
		}()
		if err := s.notifyRestaurantStaff(restaurantID, alertType, title, body, data); err != nil {
			log.Printf("push notify failed restaurant=%s type=%s: %v", restaurantID, alertType, err)
		}
	}()
}

func (s *PushService) notifyRestaurantStaff(restaurantID, alertType, title, body string, data map[string]string) error {
	var tokens []models.UserPushToken
	if err := s.db.Where("restaurant_id = ?", restaurantID).Find(&tokens).Error; err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}

	sound, channel := pushSoundForAlert(alertType)
	if data == nil {
		data = map[string]string{}
	}
	data["alert_type"] = alertType

	msgs := make([]expoPushMessage, 0, len(tokens))
	for _, t := range tokens {
		msgs = append(msgs, expoPushMessage{
			To:        t.Token,
			Title:     title,
			Body:      body,
			Sound:     sound,
			ChannelID: channel,
			Priority:  "high",
			Data:      data,
		})
	}

	payload, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, "https://exp.host/--/api/v2/push/send", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("expo push HTTP %s", resp.Status)
	}
	return nil
}

func pushSoundForAlert(alertType string) (sound, channel string) {
	switch alertType {
	case PushAlertItemsReady:
		return "items-ready.wav", "items_ready"
	case PushAlertItemCancelled:
		return "item-cancelled.wav", "item_cancelled"
	default:
		return "assistance-alert.wav", "assistance"
	}
}
