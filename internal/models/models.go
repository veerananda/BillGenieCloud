package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a staff member or restaurant owner
type User struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	RestaurantID   string    `json:"restaurant_id" gorm:"index"`
	Name           string    `json:"name" gorm:"not null"`
	Email          string    `json:"email" gorm:"index"` // Email only required for admin, nullable for staff
	Phone          string    `json:"phone"`
	PasswordHash   string    `json:"-" gorm:"not null"`
	Role           string    `json:"role" gorm:"not null;type:varchar(50)"` // "admin", "manager", "staff"
	IsActive       bool      `json:"is_active" gorm:"default:true"`
	StaffKey       string    `json:"staff_key" gorm:"unique;index"` // Globally unique per-staff key (not null enforced in migration)
	KeyGeneratedAt time.Time `json:"key_generated_at" gorm:"autoCreateTime:milli"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	Orders     []Order     `json:"-" gorm:"foreignKey:CreatedByUserID"`
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

// BeforeCreate generates UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// Restaurant represents a restaurant business
type Restaurant struct {
	ID              string          `gorm:"primaryKey" json:"id"`
	RestaurantCode  string          `json:"restaurant_code" gorm:"unique;size:10;index"` // Unique code for login (not null enforced in migration)
	Name            string          `json:"name" gorm:"not null;index"`
	OwnerName       string          `json:"owner_name"`
	Email           string          `json:"email"`
	Phone           string          `json:"phone"`
	Address         string          `json:"address"`
	City            string          `json:"city" gorm:"index"`
	Cuisine         string          `json:"cuisine"` // "Indian", "Chinese", etc.
	TotalTables     int             `json:"total_tables" gorm:"default:10"`
	TotalStaff      int             `json:"total_staff" gorm:"default:5"`
	SubscriptionEnd time.Time       `json:"subscription_end"`
	IsActive        bool            `json:"is_active" gorm:"default:true"`
	IsSelfService   bool            `json:"is_self_service" gorm:"default:false"`   // True for self-service, False for dine-in
	IsEmailVerified bool            `json:"is_email_verified" gorm:"default:false"` // Email verification status
	Settings        json.RawMessage `json:"settings" gorm:"type:jsonb"`             // Customizable settings
	// Restaurant Profile fields
	ContactNumber string    `json:"contact_number"`
	UPIQRCode     string    `json:"upi_qr_code" gorm:"type:text"` // Base64 encoded QR code
	CreatedAt     time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Users            []User            `json:"-" gorm:"foreignKey:RestaurantID"`
	Orders           []Order           `json:"-" gorm:"foreignKey:RestaurantID"`
	MenuItems        []MenuItem        `json:"-" gorm:"foreignKey:RestaurantID"`
	Inventory        []Inventory       `json:"-" gorm:"foreignKey:RestaurantID"`
	AuditLogs        []AuditLog        `json:"-" gorm:"foreignKey:RestaurantID"`
	RestaurantTables []RestaurantTable `json:"-" gorm:"foreignKey:RestaurantID"`
}

// TableName specifies the table name
func (Restaurant) TableName() string {
	return "restaurants"
}

// BeforeCreate generates UUID
func (r *Restaurant) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// Order represents a customer order
type Order struct {
	ID             string  `gorm:"primaryKey" json:"id"`
	RestaurantID   string  `json:"restaurant_id" gorm:"index" validate:"required"`
	TableNumber    string  `json:"table_number" validate:"required"`
	TableID        *string `json:"table_id" gorm:"index"` // Link to RestaurantTable for dine-in orders
	CustomerName   string  `json:"customer_name"`
	OrderNumber    int     `json:"order_number" gorm:"index"`                        // Sequential order number
	Status         string  `json:"status" gorm:"default:'pending';type:varchar(50)"` // pending, confirmed, completed, cancelled
	SubTotal       float64 `json:"sub_total" gorm:"type:numeric(10,2);default:0"`
	TaxAmount      float64 `json:"tax_amount" gorm:"type:numeric(10,2);default:0"`
	DiscountAmount float64 `json:"discount_amount" gorm:"type:numeric(10,2);default:0"`
	Total          float64 `json:"total" gorm:"type:numeric(10,2);default:0"`
	PaymentMethod  string  `json:"payment_method" gorm:"type:varchar(50)"` // "cash", "card", "upi"
	PaymentID      string  `json:"payment_id"`                             // Razorpay payment ID
	// Payment completion details
	AmountReceived  float64    `json:"amount_received,omitempty" gorm:"type:numeric(10,2)"`
	ChangeReturned  float64    `json:"change_returned,omitempty" gorm:"type:numeric(10,2)"`
	Notes           string     `json:"notes" gorm:"type:text"`
	CreatedByUserID string     `json:"created_by_user_id"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
	CompletedAt     *time.Time `json:"completed_at"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	Items      []OrderItem `json:"-" gorm:"foreignKey:OrderID;cascade:delete"`
	CreatedBy  *User       `json:"-" gorm:"foreignKey:CreatedByUserID"`
}

// TableName specifies the table name
func (Order) TableName() string {
	return "orders"
}

// BeforeCreate generates UUID
func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// OrderItem represents individual items in an order
type OrderItem struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	OrderID   string    `json:"order_id" gorm:"index" validate:"required"`
	MenuID    string    `json:"menu_id" validate:"required"`
	Quantity  int       `json:"quantity" validate:"required,min=1"`
	UnitRate  float64   `json:"unit_rate" gorm:"type:numeric(10,2)"`
	Total     float64   `json:"total" gorm:"type:numeric(10,2)"`
	Status    string    `json:"status" gorm:"default:'pending';type:varchar(50)"` // pending, preparing, ready, served
	SubId     string    `json:"sub_id,omitempty" gorm:"index"`                    // Batch tracking for incremental orders
	Notes     string    `json:"notes" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`

	// Relations
	Order    *Order    `json:"-" gorm:"foreignKey:OrderID"`
	MenuItem *MenuItem `json:"-" gorm:"foreignKey:MenuID"`
}

// TableName specifies the table name
func (OrderItem) TableName() string {
	return "order_items"
}

// BeforeCreate generates UUID
func (oi *OrderItem) BeforeCreate(tx *gorm.DB) error {
	if oi.ID == "" {
		oi.ID = uuid.New().String()
	}
	return nil
}

// MenuItem represents a food item on the menu
type MenuItem struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	RestaurantID string    `json:"restaurant_id" gorm:"index" validate:"required"`
	Name         string    `json:"name" gorm:"not null;index"`
	Category     string    `json:"category" gorm:"index"` // "appetizer", "main", "dessert", "drink"
	Description  string    `json:"description" gorm:"type:text"`
	Price        float64   `json:"price" gorm:"type:numeric(10,2);not null"`
	CostPrice    float64   `json:"cost_price" gorm:"type:numeric(10,2)"` // For margin calculation
	IsVeg        bool      `json:"is_veg" gorm:"default:false"`
	IsAvailable  bool      `json:"is_available" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	Inventory  *Inventory  `json:"-" gorm:"foreignKey:MenuItemID"`
}

// TableName specifies the table name
func (MenuItem) TableName() string {
	return "menu_items"
}

// BeforeCreate generates UUID
func (m *MenuItem) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// Inventory tracks stock levels
type Inventory struct {
	ID              string     `gorm:"primaryKey" json:"id"`
	RestaurantID    string     `json:"restaurant_id" gorm:"index" validate:"required"`
	MenuItemID      string     `json:"menu_item_id" gorm:"index" validate:"required"`
	Quantity        float64    `json:"quantity" gorm:"type:numeric(10,2);default:0"`
	Unit            string     `json:"unit" gorm:"default:'pieces'"`        // pieces, kg, liter, etc.
	MinLevel        float64    `json:"min_level" gorm:"type:numeric(10,2)"` // Alert when below this
	MaxLevel        float64    `json:"max_level" gorm:"type:numeric(10,2)"` // Reorder max
	LastRestockedAt *time.Time `json:"last_restocked_at"`
	CreatedAt       time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       time.Time  `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	MenuItem   *MenuItem   `json:"-" gorm:"foreignKey:MenuItemID"`
}

// TableName specifies the table name
func (Inventory) TableName() string {
	return "inventory"
}

// BeforeCreate generates UUID
func (inv *Inventory) BeforeCreate(tx *gorm.DB) error {
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}
	return nil
}

// Transaction represents financial transactions
type Transaction struct {
	ID              string    `gorm:"primaryKey" json:"id"`
	RestaurantID    string    `json:"restaurant_id" gorm:"index" validate:"required"`
	OrderID         string    `json:"order_id" gorm:"index"`
	Amount          float64   `json:"amount" gorm:"type:numeric(10,2);not null"`
	TransactionType string    `json:"transaction_type" gorm:"type:varchar(50)"`         // "sale", "payment", "refund", "expense"
	PaymentMethod   string    `json:"payment_method"`                                   // "cash", "card", "upi", "bank_transfer"
	PaymentID       string    `json:"payment_id"`                                       // Razorpay/external payment ID
	Status          string    `json:"status" gorm:"default:'pending';type:varchar(50)"` // pending, completed, failed
	Notes           string    `json:"notes" gorm:"type:text"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	Order      *Order      `json:"-" gorm:"foreignKey:OrderID"`
}

// TableName specifies the table name
func (Transaction) TableName() string {
	return "transactions"
}

// BeforeCreate generates UUID
func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// AuditLog tracks all important changes
type AuditLog struct {
	ID           string          `gorm:"primaryKey" json:"id"`
	RestaurantID string          `json:"restaurant_id" gorm:"index" validate:"required"`
	UserID       string          `json:"user_id"`
	Action       string          `json:"action" gorm:"index"`    // "order_created", "inventory_updated", etc.
	Entity       string          `json:"entity"`                 // "order", "inventory", "menu_item"
	EntityID     string          `json:"entity_id" gorm:"index"` // ID of the affected entity
	OldValues    json.RawMessage `json:"old_values" gorm:"type:jsonb"`
	NewValues    json.RawMessage `json:"new_values" gorm:"type:jsonb"`
	IPAddress    string          `json:"ip_address"`
	UserAgent    string          `json:"user_agent" gorm:"type:text"`
	CreatedAt    time.Time       `json:"created_at" gorm:"autoCreateTime;index"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	User       *User       `json:"-" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (AuditLog) TableName() string {
	return "audit_logs"
}

// BeforeCreate generates UUID
func (al *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if al.ID == "" {
		al.ID = uuid.New().String()
	}
	return nil
}

// NotificationEvent represents real-time WebSocket events
type NotificationEvent struct {
	Type      string          `json:"type"` // "order_created", "inventory_low", etc.
	RoomID    string          `json:"room_id"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// OrderEventData for WebSocket events
type OrderEventData struct {
	OrderID       string        `json:"order_id"`
	OrderNumber   int           `json:"order_number"`
	TableID       *string       `json:"table_id,omitempty"`
	TableNo       string        `json:"table_no"`
	TableOccupied bool          `json:"table_occupied"` // Track table occupation status
	Status        string        `json:"status"`
	SubTotal      float64       `json:"sub_total"`
	TaxAmount     float64       `json:"tax_amount"`
	TotalAmount   float64       `json:"total_amount"`
	ItemCount     int           `json:"item_count"`
	Items         []OrderItem   `json:"items,omitempty"` // Include items for detailed updates
}

// TableEventData for WebSocket table status updates
type TableEventData struct {
	TableID        string `json:"table_id"`
	TableNumber    string `json:"table_number"`
	IsOccupied     bool   `json:"is_occupied"`
	CurrentOrderID *string `json:"current_order_id,omitempty"`
}

// InventoryEventData for WebSocket inventory updates
type InventoryEventData struct {
	MenuItemID string  `json:"menu_item_id"`
	ItemName   string  `json:"item_name"`
	Quantity   float64 `json:"quantity"`
	IsLow      bool    `json:"is_low"`
	MinLevel   float64 `json:"min_level"`
}

// Ingredient represents a raw ingredient used in menu items
type Ingredient struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	RestaurantID string    `json:"restaurant_id" gorm:"index" validate:"required"`
	Name         string    `json:"name" gorm:"not null"`
	Unit         string    `json:"unit" gorm:"type:varchar(50)"` // pieces, grams, ml, liters, kg, etc.
	CurrentStock float64   `json:"current_stock" gorm:"type:numeric(10,2);default:0"`
	FullStock    float64   `json:"full_stock" gorm:"type:numeric(10,2);default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
}

// TableName specifies the table name
func (Ingredient) TableName() string {
	return "ingredients"
}

// BeforeCreate generates UUID
func (i *Ingredient) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// RestaurantTable represents a physical table in a dine-in restaurant
type RestaurantTable struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	RestaurantID   string    `json:"restaurant_id" gorm:"index" validate:"required"`
	Name           string    `json:"name" gorm:"not null;index"` // "1", "2", "1a", "VIP1", etc.
	IsOccupied     bool      `json:"is_occupied" gorm:"default:false"`
	Capacity       *int      `json:"capacity"`                      // Seating capacity - number of members
	CurrentOrderID *string   `json:"current_order_id" gorm:"index"` // Link to active order, nullable
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
	Order      *Order      `json:"-" gorm:"foreignKey:CurrentOrderID"`
}

// TableName specifies the table name
func (RestaurantTable) TableName() string {
	return "restaurant_tables"
}

// BeforeCreate generates UUID
func (rt *RestaurantTable) BeforeCreate(tx *gorm.DB) error {
	if rt.ID == "" {
		rt.ID = uuid.New().String()
	}
	return nil
}

// RefreshToken stores refresh tokens for users
type RefreshToken struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `json:"user_id" gorm:"index" validate:"required"`
	Token     string    `json:"token" gorm:"type:text;index"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	User *User `json:"-" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// BeforeCreate generates UUID
func (rt *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if rt.ID == "" {
		rt.ID = uuid.New().String()
	}
	return nil
}

// UserSession represents an active user session (tracks concurrent logins)
type UserSession struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	UserID       string    `json:"user_id" gorm:"index;not null"`
	RestaurantID string    `json:"restaurant_id" gorm:"index;not null"`
	AccessToken  string    `json:"access_token" gorm:"type:text;not null"`
	LoginTime    time.Time `json:"login_time" gorm:"autoCreateTime"`
	LastActivity time.Time `json:"last_activity" gorm:"autoUpdateTime"`
	DeviceInfo   string    `json:"device_info"` // Optional: device/app info
	IsActive     bool      `json:"is_active" gorm:"default:true"`

	// Relations
	User       *User       `json:"-" gorm:"foreignKey:UserID"`
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
}

// TableName specifies the table name
func (UserSession) TableName() string {
	return "user_sessions"
}

// BeforeCreate generates UUID
func (us *UserSession) BeforeCreate(tx *gorm.DB) error {
	if us.ID == "" {
		us.ID = uuid.New().String()
	}
	return nil
}

// PasswordReset stores password reset tokens for users
type PasswordReset struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `json:"user_id" gorm:"index;not null"`
	Email     string    `json:"email" gorm:"index"`
	Token     string    `json:"token" gorm:"type:text;uniqueIndex;not null"` // Unique reset token (hashed)
	ExpiresAt time.Time `json:"expires_at"`
	IsUsed    bool      `json:"is_used" gorm:"default:false"` // Mark as used after password reset
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Relations
	User *User `json:"-" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (PasswordReset) TableName() string {
	return "password_resets"
}

// BeforeCreate generates UUID
func (pr *PasswordReset) BeforeCreate(tx *gorm.DB) error {
	if pr.ID == "" {
		pr.ID = uuid.New().String()
	}
	return nil
}

// EmailVerification represents email verification tokens
type EmailVerification struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	RestaurantID string    `json:"restaurant_id" gorm:"index;not null"`
	Email        string    `json:"email" gorm:"not null"`
	Token        string    `json:"token" gorm:"unique;not null"`
	ExpiresAt    time.Time `json:"expires_at" gorm:"not null"`
	IsUsed       bool      `json:"is_used" gorm:"default:false"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`

	// Relations
	Restaurant *Restaurant `json:"-" gorm:"foreignKey:RestaurantID"`
}

// TableName specifies the table name
func (EmailVerification) TableName() string {
	return "email_verifications"
}

// BeforeCreate generates UUID
func (ev *EmailVerification) BeforeCreate(tx *gorm.DB) error {
	if ev.ID == "" {
		ev.ID = uuid.New().String()
	}
	return nil
}
