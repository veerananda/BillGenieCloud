package services

import "testing"

func TestCreateCustomPlanLeadValidation(t *testing.T) {
	svc := &CustomPlanLeadService{}
	_, err := svc.CreateLead(CreateCustomPlanLeadRequest{
		Name:           "",
		Phone:          "9876543210",
		RestaurantName: "Test Cafe",
		Address:        "MG Road",
	})
	if err == nil || err.Error() != "name is required" {
		t.Fatalf("expected name required, got %v", err)
	}

	_, err = svc.CreateLead(CreateCustomPlanLeadRequest{
		Name:           "Asha",
		Phone:          "9876543210",
		RestaurantName: "Test Cafe",
		Address:        "",
	})
	if err == nil || err.Error() != "address is required" {
		t.Fatalf("expected address required, got %v", err)
	}
}

func TestNormalizeLeadSourceAndStatus(t *testing.T) {
	if normalizeLeadSource("APP") != "app" {
		t.Fatal("expected app source")
	}
	if normalizeLeadSource("") != "unknown" {
		t.Fatal("expected unknown source")
	}
	if normalizeLeadStatus("contacted") != CustomPlanLeadContacted {
		t.Fatal("expected contacted")
	}
	if normalizeLeadStatus("nope") != "" {
		t.Fatal("expected empty status")
	}
}
