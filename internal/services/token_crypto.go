package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

// hashSecret digests a secret for at-rest storage (tokens, OTPs).
func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// looksLikeSHA256Hex detects already-hashed values (lazy migrate).
func looksLikeSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// findRefreshTokenByRaw looks up a refresh row by hash, with plaintext dual-read.
func findRefreshTokenByRaw(db *gorm.DB, raw string) (*models.RefreshToken, error) {
	hashed := hashSecret(raw)
	now := time.Now()
	var row models.RefreshToken
	err := db.Where("token = ? AND expires_at > ?", hashed, now).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	// Legacy plaintext rows (pre-hash deploy).
	err = db.Where("token = ? AND expires_at > ?", raw, now).First(&row).Error
	if err != nil {
		return nil, err
	}
	_ = db.Model(&row).Update("token", hashed).Error
	row.Token = hashed
	return &row, nil
}

func findPasswordResetByRaw(db *gorm.DB, raw string) (*models.PasswordReset, error) {
	hashed := hashSecret(raw)
	now := time.Now()
	var row models.PasswordReset
	err := db.Where("token = ? AND is_used = false AND expires_at > ?", hashed, now).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	err = db.Where("token = ? AND is_used = false AND expires_at > ?", raw, now).First(&row).Error
	if err != nil {
		return nil, err
	}
	_ = db.Model(&row).Update("token", hashed).Error
	row.Token = hashed
	return &row, nil
}

func findEmailVerificationByRaw(db *gorm.DB, raw string) (*models.EmailVerification, error) {
	hashed := hashSecret(raw)
	var row models.EmailVerification
	err := db.Where("token = ?", hashed).First(&row).Error
	if err == nil {
		return &row, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	err = db.Where("token = ?", raw).First(&row).Error
	if err != nil {
		return nil, err
	}
	_ = db.Model(&row).Update("token", hashed).Error
	row.Token = hashed
	return &row, nil
}

// sessionTokenMatches reports whether stored equals raw or hash(raw).
func sessionTokenMatches(stored, raw string) bool {
	if stored == "" || raw == "" {
		return false
	}
	if stored == raw {
		return true
	}
	return stored == hashSecret(raw)
}

func generateNumericOTP(length int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("otp entropy: %w", err)
		}
		b[i] = digits[n.Int64()]
	}
	return string(b), nil
}

func generateRandomToken(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("token entropy: %w", err)
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func generateSecureStaffKeySuffix(n int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[v.Int64()]
	}
	return string(b), nil
}
