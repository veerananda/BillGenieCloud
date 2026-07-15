package services

import (
	"errors"
	"strings"
	"time"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const (
	SupportIssueStatusOpen       = "open"
	SupportIssueStatusInProgress = "in_progress"
	SupportIssueStatusResolved   = "resolved"
	SupportIssueStatusClosed     = "closed"

	maxSupportScreenshotChars = 3 * 1024 * 1024
)

type CreateSupportIssueRequest struct {
	Category              string `json:"category"`
	Title                 string `json:"title"`
	Description           string `json:"description"`
	ScreenshotDataURL     string `json:"screenshot_data_url"`
	ScreenshotName        string `json:"screenshot_name"`
	ScreenshotContentType string `json:"screenshot_content_type"`
}

type UpdateSupportIssueRequest struct {
	Status         string `json:"status"`
	ResolutionNote string `json:"resolution_note"`
}

type SupportIssueSummary struct {
	ID                    string     `json:"id"`
	RestaurantID          string     `json:"restaurant_id"`
	RestaurantName        string     `json:"restaurant_name,omitempty"`
	RestaurantCode        string     `json:"restaurant_code,omitempty"`
	UserID                string     `json:"user_id,omitempty"`
	ReporterName          string     `json:"reporter_name"`
	ReporterRole          string     `json:"reporter_role"`
	Category              string     `json:"category"`
	Title                 string     `json:"title"`
	Description           string     `json:"description"`
	ScreenshotDataURL     string     `json:"screenshot_data_url,omitempty"`
	ScreenshotName        string     `json:"screenshot_name,omitempty"`
	ScreenshotContentType string     `json:"screenshot_content_type,omitempty"`
	Status                string     `json:"status"`
	ResolutionNote        string     `json:"resolution_note,omitempty"`
	ResolvedBy            string     `json:"resolved_by,omitempty"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type SupportIssueService struct {
	db *gorm.DB
}

func NewSupportIssueService(db *gorm.DB) *SupportIssueService {
	return &SupportIssueService{db: db}
}

func (s *SupportIssueService) CreateIssue(restaurantID, userID, role string, req CreateSupportIssueRequest) (*SupportIssueSummary, error) {
	if strings.TrimSpace(restaurantID) == "" {
		return nil, errors.New("restaurant_id is required")
	}

	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	if title == "" {
		return nil, errors.New("subject is required")
	}
	if description == "" {
		return nil, errors.New("description is required")
	}
	if len(title) > 160 {
		return nil, errors.New("subject must be 160 characters or less")
	}

	category := normalizeSupportCategory(req.Category)
	screenshot := strings.TrimSpace(req.ScreenshotDataURL)
	contentType := strings.TrimSpace(req.ScreenshotContentType)
	if screenshot != "" {
		if len(screenshot) > maxSupportScreenshotChars {
			return nil, errors.New("screenshot must be smaller than 3 MB")
		}
		if !strings.HasPrefix(screenshot, "data:image/") {
			return nil, errors.New("screenshot must be an image data URL")
		}
		if contentType == "" {
			contentType = inferDataURLContentType(screenshot)
		}
	}

	reporterName := ""
	var user models.User
	if userID != "" {
		if err := s.db.Where("id = ? AND restaurant_id = ?", userID, restaurantID).First(&user).Error; err == nil {
			reporterName = user.Name
			if role == "" {
				role = user.Role
			}
		}
	}

	issue := models.SupportIssue{
		RestaurantID:          restaurantID,
		UserID:                userID,
		ReporterName:          strings.TrimSpace(reporterName),
		ReporterRole:          strings.TrimSpace(role),
		Category:              category,
		Title:                 title,
		Description:           description,
		ScreenshotDataURL:     screenshot,
		ScreenshotName:        strings.TrimSpace(req.ScreenshotName),
		ScreenshotContentType: contentType,
		Status:                SupportIssueStatusOpen,
	}

	if err := s.db.Create(&issue).Error; err != nil {
		return nil, err
	}

	return s.GetIssue(issue.ID)
}

func (s *SupportIssueService) ListRestaurantIssues(restaurantID, status string, limit, offset int) ([]SupportIssueSummary, int64, error) {
	if strings.TrimSpace(restaurantID) == "" {
		return nil, 0, errors.New("restaurant_id is required")
	}

	query := s.db.Model(&models.SupportIssue{}).Where("restaurant_id = ?", restaurantID)
	if normalized := normalizeSupportStatus(status); normalized != "" {
		query = query.Where("status = ?", normalized)
	}

	return s.list(query, limit, offset)
}

func (s *SupportIssueService) ListPlatformIssues(status, search string, limit, offset int) ([]SupportIssueSummary, int64, error) {
	query := s.db.Model(&models.SupportIssue{})
	if normalized := normalizeSupportStatus(status); normalized != "" {
		query = query.Where("status = ?", normalized)
	}
	search = strings.TrimSpace(search)
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		query = query.Joins("LEFT JOIN restaurants ON restaurants.id = support_issues.restaurant_id").
			Where("LOWER(support_issues.title) LIKE ? OR LOWER(support_issues.description) LIKE ? OR LOWER(restaurants.name) LIKE ? OR LOWER(restaurants.restaurant_code) LIKE ?", like, like, like, like)
	}

	return s.list(query, limit, offset)
}

func (s *SupportIssueService) GetIssue(issueID string) (*SupportIssueSummary, error) {
	var issue models.SupportIssue
	if err := s.db.Preload("Restaurant").Where("id = ?", issueID).First(&issue).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("support issue not found")
		}
		return nil, err
	}
	summary := buildSupportIssueSummary(issue)
	return &summary, nil
}

func (s *SupportIssueService) UpdateIssue(issueID string, req UpdateSupportIssueRequest, actor string) (*SupportIssueSummary, error) {
	status := normalizeSupportStatus(req.Status)
	if status == "" {
		return nil, errors.New("valid status is required")
	}

	var issue models.SupportIssue
	if err := s.db.Where("id = ?", issueID).First(&issue).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("support issue not found")
		}
		return nil, err
	}

	now := time.Now()
	issue.Status = status
	issue.ResolutionNote = strings.TrimSpace(req.ResolutionNote)
	if status == SupportIssueStatusResolved || status == SupportIssueStatusClosed {
		issue.ResolvedBy = strings.TrimSpace(actor)
		issue.ResolvedAt = &now
	} else {
		issue.ResolvedBy = ""
		issue.ResolvedAt = nil
	}

	if err := s.db.Save(&issue).Error; err != nil {
		return nil, err
	}
	return s.GetIssue(issue.ID)
}

func (s *SupportIssueService) list(query *gorm.DB, limit, offset int) ([]SupportIssueSummary, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var issues []models.SupportIssue
	if err := query.Preload("Restaurant").
		Order("support_issues.created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&issues).Error; err != nil {
		return nil, 0, err
	}

	items := make([]SupportIssueSummary, 0, len(issues))
	for _, issue := range issues {
		items = append(items, buildSupportIssueSummary(issue))
	}
	return items, total, nil
}

func normalizeSupportCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "query", "question":
		return "query"
	case "problem", "issue", "bug":
		return "problem"
	default:
		return "other"
	}
}

func normalizeSupportStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SupportIssueStatusOpen:
		return SupportIssueStatusOpen
	case "progress", "in-progress", SupportIssueStatusInProgress:
		return SupportIssueStatusInProgress
	case SupportIssueStatusResolved:
		return SupportIssueStatusResolved
	case SupportIssueStatusClosed:
		return SupportIssueStatusClosed
	default:
		return ""
	}
}

func inferDataURLContentType(value string) string {
	if strings.HasPrefix(value, "data:image/png") {
		return "image/png"
	}
	if strings.HasPrefix(value, "data:image/webp") {
		return "image/webp"
	}
	if strings.HasPrefix(value, "data:image/gif") {
		return "image/gif"
	}
	return "image/jpeg"
}

func buildSupportIssueSummary(issue models.SupportIssue) SupportIssueSummary {
	summary := SupportIssueSummary{
		ID:                    issue.ID,
		RestaurantID:          issue.RestaurantID,
		UserID:                issue.UserID,
		ReporterName:          issue.ReporterName,
		ReporterRole:          issue.ReporterRole,
		Category:              issue.Category,
		Title:                 issue.Title,
		Description:           issue.Description,
		ScreenshotDataURL:     issue.ScreenshotDataURL,
		ScreenshotName:        issue.ScreenshotName,
		ScreenshotContentType: issue.ScreenshotContentType,
		Status:                issue.Status,
		ResolutionNote:        issue.ResolutionNote,
		ResolvedBy:            issue.ResolvedBy,
		ResolvedAt:            issue.ResolvedAt,
		CreatedAt:             issue.CreatedAt,
		UpdatedAt:             issue.UpdatedAt,
	}
	if issue.Restaurant != nil {
		summary.RestaurantName = issue.Restaurant.Name
		summary.RestaurantCode = issue.Restaurant.RestaurantCode
	}
	return summary
}
