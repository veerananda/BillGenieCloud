package services

import (
	"errors"
	"strings"
	"unicode/utf8"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const (
	CustomPlanLeadPending   = "pending"
	CustomPlanLeadContacted = "contacted"
	CustomPlanLeadConverted = "converted"
	CustomPlanLeadClosed    = "closed"
)

type CreateCustomPlanLeadRequest struct {
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	RestaurantName string `json:"restaurant_name"`
	Address        string `json:"address"`
	City           string `json:"city"`
	State          string `json:"state"`
	Notes          string `json:"notes"`
	Source         string `json:"source"`
}

type UpdateCustomPlanLeadRequest struct {
	Status       string `json:"status"`
	InternalNote string `json:"internal_note"`
}

type CustomPlanLeadSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	RestaurantName string `json:"restaurant_name"`
	Address        string `json:"address"`
	City           string `json:"city"`
	State          string `json:"state"`
	Notes          string `json:"notes,omitempty"`
	Source         string `json:"source,omitempty"`
	Status         string `json:"status"`
	InternalNote   string `json:"internal_note,omitempty"`
	UpdatedBy      string `json:"updated_by,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type CustomPlanLeadService struct {
	db *gorm.DB
}

func NewCustomPlanLeadService(db *gorm.DB) *CustomPlanLeadService {
	return &CustomPlanLeadService{db: db}
}

func (s *CustomPlanLeadService) CreateLead(req CreateCustomPlanLeadRequest) (*CustomPlanLeadSummary, error) {
	name := strings.TrimSpace(req.Name)
	phone := strings.TrimSpace(req.Phone)
	restaurant := strings.TrimSpace(req.RestaurantName)
	address := strings.TrimSpace(req.Address)
	city := strings.TrimSpace(req.City)
	state := strings.TrimSpace(req.State)
	notes := strings.TrimSpace(req.Notes)
	source := normalizeLeadSource(req.Source)

	if name == "" {
		return nil, errors.New("name is required")
	}
	if phone == "" {
		return nil, errors.New("phone is required")
	}
	if restaurant == "" {
		return nil, errors.New("restaurant_name is required")
	}
	if address == "" {
		return nil, errors.New("address is required")
	}
	if utf8.RuneCountInString(name) > 160 {
		return nil, errors.New("name must be at most 160 characters")
	}
	if utf8.RuneCountInString(phone) > 32 {
		return nil, errors.New("phone must be at most 32 characters")
	}
	if utf8.RuneCountInString(restaurant) > 200 {
		return nil, errors.New("restaurant_name must be at most 200 characters")
	}
	if utf8.RuneCountInString(notes) > 2000 {
		return nil, errors.New("notes must be at most 2000 characters")
	}

	lead := models.CustomPlanLead{
		Name:           name,
		Phone:          phone,
		RestaurantName: restaurant,
		Address:        address,
		City:           city,
		State:          state,
		Notes:          notes,
		Source:         source,
		Status:         CustomPlanLeadPending,
	}
	if err := s.db.Create(&lead).Error; err != nil {
		return nil, err
	}
	summary := buildCustomPlanLeadSummary(lead)
	return &summary, nil
}

func (s *CustomPlanLeadService) ListLeads(status, search string, limit, offset int) ([]CustomPlanLeadSummary, int64, error) {
	query := s.db.Model(&models.CustomPlanLead{})
	if normalized := normalizeLeadStatus(status); normalized != "" {
		query = query.Where("status = ?", normalized)
	}
	search = strings.TrimSpace(search)
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"name ILIKE ? OR phone ILIKE ? OR restaurant_name ILIKE ? OR city ILIKE ? OR state ILIKE ? OR address ILIKE ?",
			like, like, like, like, like, like,
		)
	}
	return s.list(query, limit, offset)
}

func (s *CustomPlanLeadService) UpdateLead(leadID string, req UpdateCustomPlanLeadRequest, actor string) (*CustomPlanLeadSummary, error) {
	status := normalizeLeadStatus(req.Status)
	if status == "" {
		return nil, errors.New("status must be pending, contacted, converted, or closed")
	}

	var lead models.CustomPlanLead
	if err := s.db.Where("id = ?", leadID).First(&lead).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("custom plan lead not found")
		}
		return nil, err
	}

	lead.Status = status
	lead.InternalNote = strings.TrimSpace(req.InternalNote)
	lead.UpdatedBy = strings.TrimSpace(actor)
	if err := s.db.Save(&lead).Error; err != nil {
		return nil, err
	}
	summary := buildCustomPlanLeadSummary(lead)
	return &summary, nil
}

func (s *CustomPlanLeadService) list(query *gorm.DB, limit, offset int) ([]CustomPlanLeadSummary, int64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var leads []models.CustomPlanLead
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&leads).Error; err != nil {
		return nil, 0, err
	}

	items := make([]CustomPlanLeadSummary, 0, len(leads))
	for _, lead := range leads {
		items = append(items, buildCustomPlanLeadSummary(lead))
	}
	return items, total, nil
}

func normalizeLeadSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "app":
		return "app"
	case "web":
		return "web"
	default:
		return "unknown"
	}
}

func normalizeLeadStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case CustomPlanLeadPending:
		return CustomPlanLeadPending
	case CustomPlanLeadContacted:
		return CustomPlanLeadContacted
	case CustomPlanLeadConverted:
		return CustomPlanLeadConverted
	case CustomPlanLeadClosed:
		return CustomPlanLeadClosed
	default:
		return ""
	}
}

func buildCustomPlanLeadSummary(lead models.CustomPlanLead) CustomPlanLeadSummary {
	return CustomPlanLeadSummary{
		ID:             lead.ID,
		Name:           lead.Name,
		Phone:          lead.Phone,
		RestaurantName: lead.RestaurantName,
		Address:        lead.Address,
		City:           lead.City,
		State:          lead.State,
		Notes:          lead.Notes,
		Source:         lead.Source,
		Status:         lead.Status,
		InternalNote:   lead.InternalNote,
		UpdatedBy:      lead.UpdatedBy,
		CreatedAt:      lead.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      lead.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}
