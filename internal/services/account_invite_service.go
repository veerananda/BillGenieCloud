package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"restaurant-api/internal/models"

	"gorm.io/gorm"
)

const (
	AccountInviteRequested  = "requested"
	AccountInvitePriced     = "priced"
	AccountInviteRegistered = "registered"
	AccountInviteClosed     = "closed"
	registerTokenTTL        = 14 * 24 * time.Hour
)

type CreateAccountRequestInput struct {
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	RestaurantName string `json:"restaurant_name"`
	Address        string `json:"address"`
	City           string `json:"city"`
	State          string `json:"state"`
	Notes          string `json:"notes"`
	Source         string `json:"source"`
}

type SetAccountInviteDealInput struct {
	Reason               string `json:"reason"`
	MonthlyPrice         int    `json:"monthly_price"`
	AnnualPrice          int    `json:"annual_price"`
	MaxTables            int    `json:"max_tables"`
	ExtraStaff           int    `json:"extra_staff"`
	ExtraChefs           int    `json:"extra_chefs"`
	ExtraManagers        int    `json:"extra_managers"`
	Inventory            bool   `json:"inventory"`
	Expenses             bool   `json:"expenses"`
	HistoryExtended      bool   `json:"history_extended"`
	LockSelfServeChanges bool   `json:"lock_self_serve_changes"`
	DealNotes            string `json:"deal_notes"`
	InternalNote         string `json:"internal_note"`
}

type AccountInviteSummary struct {
	ID                     string  `json:"id"`
	LoginID                string  `json:"login_id"`
	Name                   string  `json:"name"`
	Phone                  string  `json:"phone"`
	RestaurantName         string  `json:"restaurant_name"`
	Address                string  `json:"address"`
	City                   string  `json:"city"`
	State                  string  `json:"state"`
	Notes                  string  `json:"notes,omitempty"`
	Source                 string  `json:"source,omitempty"`
	Status                 string  `json:"status"`
	InternalNote           string  `json:"internal_note,omitempty"`
	MonthlyPrice           int     `json:"monthly_price"`
	AnnualPrice            int     `json:"annual_price"`
	MaxTables              int     `json:"max_tables"`
	ExtraStaff             int     `json:"extra_staff"`
	ExtraChefs             int     `json:"extra_chefs"`
	ExtraManagers          int     `json:"extra_managers"`
	Inventory              bool    `json:"inventory"`
	Expenses               bool    `json:"expenses"`
	HistoryExtended        bool    `json:"history_extended"`
	LockSelfServeChanges   bool    `json:"lock_self_serve_changes"`
	DealNotes              string  `json:"deal_notes,omitempty"`
	HasRegisterToken       bool    `json:"has_register_token"`
	RegisterTokenExpiresAt *string `json:"register_token_expires_at,omitempty"`
	RestaurantID           string  `json:"restaurant_id,omitempty"`
	UpdatedBy              string  `json:"updated_by,omitempty"`
	CreatedAt              string  `json:"created_at"`
	UpdatedAt              string  `json:"updated_at"`
}

type RegisterInvitePreview struct {
	LoginID         string         `json:"login_id"`
	RestaurantName  string         `json:"restaurant_name"`
	Name            string         `json:"name"`
	Phone           string         `json:"phone"`
	Address         string         `json:"address"`
	City            string         `json:"city"`
	State           string         `json:"state"`
	MaxTables       int            `json:"max_tables"`
	ExtraStaff      int            `json:"extra_staff"`
	ExtraChefs      int            `json:"extra_chefs"`
	ExtraManagers   int            `json:"extra_managers"`
	Inventory       bool           `json:"inventory"`
	Expenses        bool           `json:"expenses"`
	HistoryExtended bool           `json:"history_extended"`
	MonthlyPrice    int            `json:"monthly_price"`
	AnnualPrice     int            `json:"annual_price"`
	CyclePrices     map[string]int `json:"cycle_prices"` // period subtotal excl. GST
}

type AccountInviteService struct {
	db *gorm.DB
}

func NewAccountInviteService(db *gorm.DB) *AccountInviteService {
	return &AccountInviteService{db: db}
}

func (s *AccountInviteService) CreateRequest(req CreateAccountRequestInput) (*AccountInviteSummary, error) {
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

	loginID, err := s.allocateLoginID()
	if err != nil {
		return nil, err
	}

	invite := models.AccountInvite{
		LoginID:        loginID,
		Name:           name,
		Phone:          phone,
		RestaurantName: restaurant,
		Address:        address,
		City:           city,
		State:          state,
		Notes:          notes,
		Source:         source,
		Status:         AccountInviteRequested,
	}
	if err := s.db.Create(&invite).Error; err != nil {
		return nil, err
	}

	go s.notifyOpsNewRequest(invite)

	summary := buildAccountInviteSummary(invite)
	return &summary, nil
}

func (s *AccountInviteService) List(status, search string, limit, offset int) ([]AccountInviteSummary, int64, error) {
	query := s.db.Model(&models.AccountInvite{})
	if status = strings.TrimSpace(strings.ToLower(status)); status != "" {
		query = query.Where("status = ?", status)
	}
	search = strings.TrimSpace(search)
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(
			"login_id ILIKE ? OR name ILIKE ? OR phone ILIKE ? OR restaurant_name ILIKE ?",
			like, like, like, like,
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var rows []models.AccountInvite
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]AccountInviteSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, buildAccountInviteSummary(row))
	}
	return out, total, nil
}

func (s *AccountInviteService) GetByID(id string) (*AccountInviteSummary, error) {
	var invite models.AccountInvite
	if err := s.db.Where("id = ?", id).First(&invite).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("account invite not found")
		}
		return nil, err
	}
	summary := buildAccountInviteSummary(invite)
	return &summary, nil
}

// SetDealAndIssueToken prices the invite and returns a one-time plaintext register token.
func (s *AccountInviteService) SetDealAndIssueToken(id string, req SetAccountInviteDealInput, actor string) (*AccountInviteSummary, string, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, "", errors.New("reason is required")
	}
	if req.MonthlyPrice < 1 {
		return nil, "", errors.New("monthly_price must be at least 1")
	}
	if req.MaxTables < 1 {
		req.MaxTables = PlanStarterTables
	}
	if req.MaxTables > MaxTablesCustomDeal {
		req.MaxTables = MaxTablesCustomDeal
	}
	annual := req.AnnualPrice
	if annual <= 0 {
		annual = req.MonthlyPrice * 11
	}

	var invite models.AccountInvite
	if err := s.db.Where("id = ?", id).First(&invite).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, "", errors.New("account invite not found")
		}
		return nil, "", err
	}
	if invite.Status == AccountInviteRegistered {
		return nil, "", errors.New("invite already registered")
	}
	if invite.Status == AccountInviteClosed {
		return nil, "", errors.New("invite is closed")
	}

	token, err := generateRegisterToken()
	if err != nil {
		return nil, "", err
	}
	expires := time.Now().Add(registerTokenTTL)

	invite.MonthlyPrice = req.MonthlyPrice
	invite.AnnualPrice = annual
	invite.MaxTables = req.MaxTables
	invite.ExtraStaff = clampCount(req.ExtraStaff, 100)
	invite.ExtraChefs = clampCount(req.ExtraChefs, 50)
	invite.ExtraManagers = clampCount(req.ExtraManagers, 50)
	invite.Inventory = req.Inventory
	invite.Expenses = req.Expenses
	invite.HistoryExtended = req.HistoryExtended
	invite.LockSelfServeChanges = req.LockSelfServeChanges
	invite.DealNotes = strings.TrimSpace(req.DealNotes)
	if note := strings.TrimSpace(req.InternalNote); note != "" {
		invite.InternalNote = note
	}
	invite.RegisterTokenHash = hashRegisterToken(token)
	invite.RegisterTokenExpiresAt = &expires
	invite.Status = AccountInvitePriced
	invite.UpdatedBy = strings.TrimSpace(actor)

	if err := s.db.Save(&invite).Error; err != nil {
		return nil, "", err
	}
	summary := buildAccountInviteSummary(invite)
	return &summary, token, nil
}

func (s *AccountInviteService) PreviewRegister(loginID, rawToken string) (*RegisterInvitePreview, error) {
	invite, err := s.loadPricedInvite(loginID, rawToken)
	if err != nil {
		return nil, err
	}
	return buildRegisterInvitePreview(*invite), nil
}

func (s *AccountInviteService) loadPricedInvite(loginID, rawToken string) (*models.AccountInvite, error) {
	loginID = strings.TrimSpace(loginID)
	rawToken = strings.TrimSpace(rawToken)
	if loginID == "" || rawToken == "" {
		return nil, errors.New("login_id and token are required")
	}
	var invite models.AccountInvite
	if err := s.db.Where("login_id = ?", loginID).First(&invite).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("invalid login id or token")
		}
		return nil, err
	}
	if invite.Status != AccountInvitePriced {
		return nil, errors.New("this invite is not ready for registration")
	}
	if invite.RegisterTokenHash == "" || invite.RegisterTokenExpiresAt == nil {
		return nil, errors.New("register token has not been issued")
	}
	if time.Now().After(*invite.RegisterTokenExpiresAt) {
		return nil, errors.New("register token has expired — ask BillGenie for a new one")
	}
	if hashRegisterToken(rawToken) != invite.RegisterTokenHash {
		return nil, errors.New("invalid login id or token")
	}
	return &invite, nil
}

func (s *AccountInviteService) MarkRegistered(inviteID, restaurantID string) error {
	restaurantID = strings.TrimSpace(restaurantID)
	updates := map[string]interface{}{
		"status":                    AccountInviteRegistered,
		"register_token_hash":       "",
		"register_token_expires_at": gorm.Expr("NULL"),
	}
	if restaurantID != "" {
		updates["restaurant_id"] = restaurantID
	}
	return s.db.Model(&models.AccountInvite{}).
		Where("id = ?", inviteID).
		Updates(updates).Error
}

func (s *AccountInviteService) allocateLoginID() (string, error) {
	for i := 0; i < 40; i++ {
		suffixBytes := make([]byte, 3)
		if _, err := rand.Read(suffixBytes); err != nil {
			return "", err
		}
		n := int(suffixBytes[0])<<16 | int(suffixBytes[1])<<8 | int(suffixBytes[2])
		n = n % 100000
		loginID := fmt.Sprintf("100%05d", n)

		var existingUser models.User
		if err := s.db.Where("staff_key = ?", loginID).First(&existingUser).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return "", err
		}
		var existingInvite models.AccountInvite
		if err := s.db.Where("login_id = ?", loginID).First(&existingInvite).Error; err == nil {
			continue
		} else if err != gorm.ErrRecordNotFound {
			return "", err
		}
		return loginID, nil
	}
	return "", errors.New("could not allocate a login id — try again")
}

func (s *AccountInviteService) notifyOpsNewRequest(invite models.AccountInvite) {
	to := platformOpsNotifyEmail()
	subject := fmt.Sprintf("BillGenie account request — login %s", invite.LoginID)
	body := fmt.Sprintf(
		"New account request\n\nLogin ID: %s\nName: %s\nPhone: %s\nRestaurant: %s\nAddress: %s\nCity: %s\nState: %s\nSource: %s\nNotes: %s\n\nSet pricing in platform, then share login ID + register token with the customer.\n",
		invite.LoginID, invite.Name, invite.Phone, invite.RestaurantName, invite.Address, invite.City, invite.State, invite.Source, invite.Notes,
	)
	if err := sendEmailSMTP(to, subject, body); err != nil {
		fmt.Printf("⚠️ account request ops email failed: %v\n", err)
	}
}

func buildAccountInviteSummary(invite models.AccountInvite) AccountInviteSummary {
	out := AccountInviteSummary{
		ID:                   invite.ID,
		LoginID:              invite.LoginID,
		Name:                 invite.Name,
		Phone:                invite.Phone,
		RestaurantName:       invite.RestaurantName,
		Address:              invite.Address,
		City:                 invite.City,
		State:                invite.State,
		Notes:                invite.Notes,
		Source:               invite.Source,
		Status:               invite.Status,
		InternalNote:         invite.InternalNote,
		MonthlyPrice:         invite.MonthlyPrice,
		AnnualPrice:          invite.AnnualPrice,
		MaxTables:            invite.MaxTables,
		ExtraStaff:           invite.ExtraStaff,
		ExtraChefs:           invite.ExtraChefs,
		ExtraManagers:        invite.ExtraManagers,
		Inventory:            invite.Inventory,
		Expenses:             invite.Expenses,
		HistoryExtended:      invite.HistoryExtended,
		LockSelfServeChanges: invite.LockSelfServeChanges,
		DealNotes:            invite.DealNotes,
		HasRegisterToken:     invite.RegisterTokenHash != "",
		UpdatedBy:            invite.UpdatedBy,
		CreatedAt:            invite.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            invite.UpdatedAt.Format(time.RFC3339),
	}
	if invite.RestaurantID != nil {
		out.RestaurantID = *invite.RestaurantID
	}
	if invite.RegisterTokenExpiresAt != nil {
		s := invite.RegisterTokenExpiresAt.Format(time.RFC3339)
		out.RegisterTokenExpiresAt = &s
	}
	return out
}

func buildRegisterInvitePreview(invite models.AccountInvite) *RegisterInvitePreview {
	annual := invite.AnnualPrice
	if annual <= 0 {
		annual = invite.MonthlyPrice * 11
	}
	return &RegisterInvitePreview{
		LoginID:         invite.LoginID,
		RestaurantName:  invite.RestaurantName,
		Name:            invite.Name,
		Phone:           invite.Phone,
		Address:         invite.Address,
		City:            invite.City,
		State:           invite.State,
		MaxTables:       invite.MaxTables,
		ExtraStaff:      invite.ExtraStaff,
		ExtraChefs:      invite.ExtraChefs,
		ExtraManagers:   invite.ExtraManagers,
		Inventory:       invite.Inventory,
		Expenses:        invite.Expenses,
		HistoryExtended: invite.HistoryExtended,
		MonthlyPrice:    invite.MonthlyPrice,
		AnnualPrice:     annual,
		CyclePrices: map[string]int{
			BillingCycleQuarterly:  invite.MonthlyPrice * 3,
			BillingCycleHalfYearly: invite.MonthlyPrice * 6,
			BillingCycleAnnual:     annual,
		},
	}
}

func generateRegisterToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashRegisterToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func inviteDealFromModel(invite models.AccountInvite, billingCycle string) (CustomDeal, error) {
	cycle := NormalizeBillingCycle(billingCycle)
	if cycle == "" {
		cycle = BillingCycleQuarterly
	}
	annual := invite.AnnualPrice
	if annual <= 0 {
		annual = invite.MonthlyPrice * 11
	}
	deal := CustomDeal{
		MonthlyPrice: invite.MonthlyPrice,
		AnnualPrice:  annual,
		Selection: SubscriptionSelection{
			BillingCycle:    cycle,
			OperationMode:   "both",
			MaxTables:       invite.MaxTables,
			ExtraStaff:      invite.ExtraStaff,
			ExtraChefs:      invite.ExtraChefs,
			ExtraManagers:   invite.ExtraManagers,
			HistoryExtended: invite.HistoryExtended,
			Inventory:       invite.Inventory,
			Expenses:        invite.Expenses,
			KitchenDineIn:   true,
			KitchenCounter:  true,
		},
		LockSelfServeChanges: invite.LockSelfServeChanges,
		Notes:                invite.DealNotes,
	}
	return ValidateCustomDeal(deal)
}
