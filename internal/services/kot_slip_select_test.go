package services

import (
	"testing"

	"restaurant-api/internal/models"
)

func TestSelectKOTSlipItems_FirstFireIsNotAddOn(t *testing.T) {
	items := []models.OrderItem{
		{SubId: "batch-a", Status: "pending", Quantity: 1},
		{SubId: "batch-a", Status: "pending", Quantity: 2},
	}

	got, isAddOn := selectKOTSlipItems(items, true)
	if isAddOn {
		t.Fatal("expected first kitchen fire not to be ADD-ON")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}

	got, isAddOn = selectKOTSlipItems(items, false)
	if isAddOn {
		t.Fatal("expected create-order fire not to be ADD-ON")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}
}

func TestSelectKOTSlipItems_SecondFireIsAddOn(t *testing.T) {
	items := []models.OrderItem{
		{SubId: "batch-a", Status: "pending", Quantity: 1},
		{SubId: "batch-b", Status: "pending", Quantity: 1},
		{SubId: "batch-b", Status: "preparing", Quantity: 2},
	}

	got, isAddOn := selectKOTSlipItems(items, true)
	if !isAddOn {
		t.Fatal("expected later kitchen fire to be ADD-ON")
	}
	if len(got) != 2 {
		t.Fatalf("expected only latest batch (2 lines), got %d", len(got))
	}
	for _, it := range got {
		if it.SubId != "batch-b" {
			t.Fatalf("expected only batch-b lines, got %q", it.SubId)
		}
	}
}

func TestSelectKOTSlipItems_IgnoresCancelledPriorBatch(t *testing.T) {
	items := []models.OrderItem{
		{SubId: "batch-a", Status: "cancelled", Quantity: 1},
		{SubId: "batch-b", Status: "pending", Quantity: 1},
	}

	got, isAddOn := selectKOTSlipItems(items, true)
	if isAddOn {
		t.Fatal("cancelled prior batch should not make this an ADD-ON")
	}
	if len(got) != 1 || got[0].SubId != "batch-b" {
		t.Fatalf("unexpected selection: %+v", got)
	}
}
