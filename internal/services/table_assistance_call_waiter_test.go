package services

import (
	"errors"
	"testing"
	"time"

	"restaurant-api/internal/models"
)

func TestRequestTableAssistanceRejectsVacant(t *testing.T) {
	table := &models.RestaurantTable{IsOccupied: false}
	newly, err := RequestTableAssistance(nil, table)
	if newly {
		t.Fatal("expected newlyRequested=false")
	}
	if !errors.Is(err, ErrTableVacant) {
		t.Fatalf("expected ErrTableVacant, got %v", err)
	}
}

func TestRequestTableAssistanceIdempotentWhenAlreadyRequested(t *testing.T) {
	at := time.Now()
	table := &models.RestaurantTable{IsOccupied: true, AssistanceRequestedAt: &at}
	newly, err := RequestTableAssistance(nil, table)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if newly {
		t.Fatal("expected newlyRequested=false when already requested")
	}
}
