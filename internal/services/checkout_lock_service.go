package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var ErrCheckoutInProgress = errors.New("checkout already in progress on another device")

type CheckoutLock struct {
	OrderID      string    `json:"order_id"`
	RestaurantID string    `json:"restaurant_id"`
	UserID       string    `json:"user_id"`
	UserName     string    `json:"user_name"`
	LockedAt     time.Time `json:"locked_at"`
}

type CheckoutLockService struct {
	db    *gorm.DB
	redis *redis.Client
	local sync.Map
}

func NewCheckoutLockService(db *gorm.DB) *CheckoutLockService {
	svc := &CheckoutLockService{db: db}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return svc
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return svc
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return svc
	}

	svc.redis = client
	return svc
}

func checkoutLockKey(orderID string) string {
	return fmt.Sprintf("billgenie:checkout:%s", orderID)
}

const checkoutLockTTL = 15 * time.Minute

func (s *CheckoutLockService) resolveUserName(userID string) string {
	if s.db == nil || userID == "" {
		return "Staff"
	}
	var name string
	if err := s.db.Table("users").Select("name").Where("id = ?", userID).Scan(&name).Error; err != nil || name == "" {
		return "Staff"
	}
	return name
}

func (s *CheckoutLockService) Acquire(orderID, restaurantID, userID string) (*CheckoutLock, error) {
	lock := &CheckoutLock{
		OrderID:      orderID,
		RestaurantID: restaurantID,
		UserID:       userID,
		UserName:     s.resolveUserName(userID),
		LockedAt:     time.Now(),
	}

	payload, err := json.Marshal(lock)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		key := checkoutLockKey(orderID)
		ok, err := s.redis.SetNX(ctx, key, payload, checkoutLockTTL).Result()
		if err != nil {
			return nil, err
		}
		if !ok {
			existing, getErr := s.getRedisLock(orderID)
			if getErr != nil {
				return nil, ErrCheckoutInProgress
			}
			if existing.UserID == userID {
				_ = s.redis.Set(ctx, key, payload, checkoutLockTTL).Err()
				return lock, nil
			}
			return existing, ErrCheckoutInProgress
		}
		return lock, nil
	}

	if raw, loaded := s.local.Load(orderID); loaded {
		existing := raw.(CheckoutLock)
		if existing.UserID != userID && time.Now().Before(existing.LockedAt.Add(checkoutLockTTL)) {
			return &existing, ErrCheckoutInProgress
		}
	}
	s.local.Store(orderID, *lock)
	return lock, nil
}

func (s *CheckoutLockService) getRedisLock(orderID string) (*CheckoutLock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	raw, err := s.redis.Get(ctx, checkoutLockKey(orderID)).Bytes()
	if err != nil {
		return nil, err
	}
	var lock CheckoutLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, err
	}
	return &lock, nil
}

func (s *CheckoutLockService) Release(orderID, userID string) {
	if s.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		existing, err := s.getRedisLock(orderID)
		if err != nil || existing.UserID != userID {
			return
		}
		_ = s.redis.Del(ctx, checkoutLockKey(orderID)).Err()
		return
	}

	if raw, loaded := s.local.Load(orderID); loaded {
		existing := raw.(CheckoutLock)
		if existing.UserID == userID {
			s.local.Delete(orderID)
		}
	}
}

func (s *CheckoutLockService) ForceRelease(orderID string) {
	if s.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.redis.Del(ctx, checkoutLockKey(orderID)).Err()
		return
	}
	s.local.Delete(orderID)
}
