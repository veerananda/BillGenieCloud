package services

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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

	maxSupportScreenshotChars     = 700_000 // ~500 KB image as a base64 data URL
	maxSupportScreenshots         = 5
	supportScreenshotRetentionDays = 90
)

type SupportIssueScreenshot struct {
	DataURL     string `json:"data_url"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
}

type CreateSupportIssueRequest struct {
	Category              string                   `json:"category"`
	Title                 string                   `json:"title"`
	Description           string                   `json:"description"`
	Screenshots           []SupportIssueScreenshot `json:"screenshots"`
	ScreenshotDataURL     string                   `json:"screenshot_data_url"`
	ScreenshotName        string                   `json:"screenshot_name"`
	ScreenshotContentType string                   `json:"screenshot_content_type"`
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
	ScreenshotCount       int        `json:"screenshot_count"`
	ScreenshotDataURL     string                   `json:"screenshot_data_url,omitempty"`
	ScreenshotName        string                   `json:"screenshot_name,omitempty"`
	ScreenshotContentType string                   `json:"screenshot_content_type,omitempty"`
	Screenshots           []SupportIssueScreenshot `json:"screenshots,omitempty"`
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
	screenshots, err := normalizeSupportScreenshots(req)
	if err != nil {
		return nil, err
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
		Status:                SupportIssueStatusOpen,
	}
	if len(screenshots) > 0 {
		issue.ScreenshotDataURL = screenshots[0].DataURL
		issue.ScreenshotName = screenshots[0].Name
		issue.ScreenshotContentType = screenshots[0].ContentType
		if raw, err := json.Marshal(screenshots); err == nil {
			issue.Screenshots = raw
		}
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

func (s *SupportIssueService) GetRestaurantIssueScreenshots(restaurantID, issueID string) ([]SupportIssueScreenshot, error) {
	var issue models.SupportIssue
	if err := s.db.Where("id = ? AND restaurant_id = ?", issueID, restaurantID).First(&issue).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("support issue not found")
		}
		return nil, err
	}
	return parseIssueScreenshots(issue), nil
}

func (s *SupportIssueService) GetPlatformIssueScreenshots(issueID string) ([]SupportIssueScreenshot, error) {
	var issue models.SupportIssue
	if err := s.db.Where("id = ?", issueID).First(&issue).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("support issue not found")
		}
		return nil, err
	}
	return parseIssueScreenshots(issue), nil
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
		items = append(items, buildSupportIssueListSummary(issue))
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

func normalizeSupportScreenshots(req CreateSupportIssueRequest) ([]SupportIssueScreenshot, error) {
	screenshots := make([]SupportIssueScreenshot, 0, len(req.Screenshots)+1)
	for _, item := range req.Screenshots {
		normalized, err := validateSupportScreenshot(item.DataURL, item.Name, item.ContentType)
		if err != nil {
			return nil, err
		}
		if normalized.DataURL != "" {
			screenshots = append(screenshots, normalized)
		}
	}

	legacyScreenshot := strings.TrimSpace(req.ScreenshotDataURL)
	if legacyScreenshot != "" {
		normalized, err := validateSupportScreenshot(legacyScreenshot, req.ScreenshotName, req.ScreenshotContentType)
		if err != nil {
			return nil, err
		}
		if normalized.DataURL != "" {
			alreadyIncluded := false
			for _, existing := range screenshots {
				if existing.DataURL == normalized.DataURL {
					alreadyIncluded = true
					break
				}
			}
			if !alreadyIncluded {
				screenshots = append(screenshots, normalized)
			}
		}
	}

	if len(screenshots) > maxSupportScreenshots {
		return nil, errors.New("you can attach up to 5 screenshots")
	}
	return screenshots, nil
}

func validateSupportScreenshot(dataURL, name, contentType string) (SupportIssueScreenshot, error) {
	dataURL = strings.TrimSpace(dataURL)
	if dataURL == "" {
		return SupportIssueScreenshot{}, nil
	}
	if len(dataURL) > maxSupportScreenshotChars {
		return SupportIssueScreenshot{}, errors.New("each screenshot must be smaller than 500 KB")
	}
	if !strings.HasPrefix(dataURL, "data:image/") {
		return SupportIssueScreenshot{}, errors.New("screenshots must be image data URLs")
	}

	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = inferDataURLContentType(dataURL)
	}

	screenshotName := strings.TrimSpace(name)
	if screenshotName == "" {
		screenshotName = "support-screenshot.webp"
	}

	return SupportIssueScreenshot{
		DataURL:     dataURL,
		Name:        screenshotName,
		ContentType: contentType,
	}, nil
}

func parseIssueScreenshots(issue models.SupportIssue) []SupportIssueScreenshot {
	if len(issue.Screenshots) > 0 {
		var screenshots []SupportIssueScreenshot
		if err := json.Unmarshal(issue.Screenshots, &screenshots); err == nil && len(screenshots) > 0 {
			return screenshots
		}
	}

	if strings.TrimSpace(issue.ScreenshotDataURL) != "" {
		return []SupportIssueScreenshot{{
			DataURL:     issue.ScreenshotDataURL,
			Name:        issue.ScreenshotName,
			ContentType: issue.ScreenshotContentType,
		}}
	}

	return nil
}

func buildSupportIssueSummary(issue models.SupportIssue) SupportIssueSummary {
	screenshots := parseIssueScreenshots(issue)
	summary := SupportIssueSummary{
		ID:           issue.ID,
		RestaurantID: issue.RestaurantID,
		UserID:       issue.UserID,
		ReporterName: issue.ReporterName,
		ReporterRole: issue.ReporterRole,
		Category:     issue.Category,
		Title:        issue.Title,
		Description:  issue.Description,
		ScreenshotCount: func() int {
			if screenshots == nil {
				return 0
			}
			return len(screenshots)
		}(),
		Screenshots:  screenshots,
		Status:       issue.Status,
		ResolutionNote: issue.ResolutionNote,
		ResolvedBy:     issue.ResolvedBy,
		ResolvedAt:     issue.ResolvedAt,
		CreatedAt:      issue.CreatedAt,
		UpdatedAt:      issue.UpdatedAt,
	}
	if len(screenshots) > 0 {
		summary.ScreenshotDataURL = screenshots[0].DataURL
		summary.ScreenshotName = screenshots[0].Name
		summary.ScreenshotContentType = screenshots[0].ContentType
	}
	if issue.Restaurant != nil {
		summary.RestaurantName = issue.Restaurant.Name
		summary.RestaurantCode = issue.Restaurant.RestaurantCode
	}
	return summary
}

func countIssueScreenshots(issue models.SupportIssue) int {
	if len(issue.Screenshots) > 0 {
		var screenshots []SupportIssueScreenshot
		if err := json.Unmarshal(issue.Screenshots, &screenshots); err == nil {
			return len(screenshots)
		}
	}
	if strings.TrimSpace(issue.ScreenshotDataURL) != "" {
		return 1
	}
	return 0
}

// buildSupportIssueListSummary intentionally omits screenshot data URLs to keep list responses small.
func buildSupportIssueListSummary(issue models.SupportIssue) SupportIssueSummary {
	count := countIssueScreenshots(issue)
	summary := SupportIssueSummary{
		ID:              issue.ID,
		RestaurantID:    issue.RestaurantID,
		UserID:          issue.UserID,
		ReporterName:    issue.ReporterName,
		ReporterRole:    issue.ReporterRole,
		Category:        issue.Category,
		Title:           issue.Title,
		Description:     issue.Description,
		ScreenshotCount: count,
		Status:          issue.Status,
		ResolutionNote:  issue.ResolutionNote,
		ResolvedBy:      issue.ResolvedBy,
		ResolvedAt:      issue.ResolvedAt,
		CreatedAt:       issue.CreatedAt,
		UpdatedAt:       issue.UpdatedAt,
		// ScreenshotDataURL / Screenshots are left empty to enable lazy loading.
	}
	if issue.Restaurant != nil {
		summary.RestaurantName = issue.Restaurant.Name
		summary.RestaurantCode = issue.Restaurant.RestaurantCode
	}
	return summary
}

// CleanupOldSupportScreenshots removes screenshot payloads from resolved/closed tickets
// older than supportScreenshotRetentionDays to reduce database bloat.
func (s *SupportIssueService) CleanupOldSupportScreenshots() (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -supportScreenshotRetentionDays)
	result := s.db.Model(&models.SupportIssue{}).
		Where("status IN ?", []string{SupportIssueStatusResolved, SupportIssueStatusClosed}).
		Where(
			"(resolved_at IS NOT NULL AND resolved_at < ?) OR (resolved_at IS NULL AND updated_at < ?)",
			cutoff,
			cutoff,
		).
		Where(
			"(screenshot_data_url <> '' AND screenshot_data_url IS NOT NULL) OR (screenshots IS NOT NULL AND screenshots::text NOT IN ('null', '[]', ''))",
		).
		Updates(map[string]interface{}{
			"screenshot_data_url":     "",
			"screenshot_name":         "",
			"screenshot_content_type": "",
			"screenshots":             json.RawMessage("[]"),
			"updated_at":              time.Now(),
		})
	return result.RowsAffected, result.Error
}

// StartScreenshotRetentionCleanup runs daily cleanup of old support screenshots.
func (s *SupportIssueService) StartScreenshotRetentionCleanup(ctx context.Context) {
	run := func() {
		if count, err := s.CleanupOldSupportScreenshots(); err != nil {
			log.Printf("support screenshot retention cleanup failed: %v", err)
		} else if count > 0 {
			log.Printf("cleared screenshots from %d closed support issue(s)", count)
		}
	}

	run()

	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}
