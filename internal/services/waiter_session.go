package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const waiterSessionPurpose = "table_waiter"

type waiterSessionClaims struct {
	Purpose string `json:"purpose"`
	TableID string `json:"tid"`
	OrderID string `json:"oid"`
	jwt.RegisteredClaims
}

func waiterSessionSecret() []byte {
	s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if s == "" {
		s = "dev-insecure-jwt-secret-change-me"
	}
	return []byte(s)
}

// DeriveAssistanceUnlockCode returns a 4-digit code for the live order.
// Staff shows it; the guest enters it once on the QR page to unlock Call waiter.
func DeriveAssistanceUnlockCode(orderID string) string {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return ""
	}
	mac := hmac.New(sha256.New, waiterSessionSecret())
	_, _ = mac.Write([]byte("assistance-unlock:" + orderID))
	sum := mac.Sum(nil)
	n := binary.BigEndian.Uint32(sum[:4]) % 10000
	return fmt.Sprintf("%04d", n)
}

// IssueWaiterSession mints a token bound to the current table + dine-in order.
func IssueWaiterSession(tableID, orderID string) (string, error) {
	tableID = strings.TrimSpace(tableID)
	orderID = strings.TrimSpace(orderID)
	if tableID == "" || orderID == "" {
		return "", errors.New("table and order required")
	}
	claims := waiterSessionClaims{
		Purpose: waiterSessionPurpose,
		TableID: tableID,
		OrderID: orderID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(waiterSessionSecret())
}

// ValidateWaiterSession ensures the token matches this table's current order.
func ValidateWaiterSession(raw, tableID, currentOrderID string) error {
	raw = strings.TrimSpace(raw)
	tableID = strings.TrimSpace(tableID)
	currentOrderID = strings.TrimSpace(currentOrderID)
	if raw == "" || tableID == "" || currentOrderID == "" {
		return ErrWaiterSessionInvalid
	}
	parsed, err := jwt.ParseWithClaims(raw, &waiterSessionClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, ErrWaiterSessionInvalid
		}
		return waiterSessionSecret(), nil
	})
	if err != nil || !parsed.Valid {
		return ErrWaiterSessionInvalid
	}
	claims, ok := parsed.Claims.(*waiterSessionClaims)
	if !ok || claims.Purpose != waiterSessionPurpose {
		return ErrWaiterSessionInvalid
	}
	if claims.TableID != tableID || claims.OrderID != currentOrderID {
		return ErrWaiterSessionInvalid
	}
	return nil
}
