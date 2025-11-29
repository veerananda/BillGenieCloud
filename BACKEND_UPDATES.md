# Backend API Updates for Restaurant Profile & Payment Completion

## Overview
Added backend support for restaurant profile management and payment completion features that were implemented in the frontend.

## Database Model Changes

### 1. Restaurant Model (`models.go`)
**Added Fields:**
- `ContactNumber string` - Restaurant contact number for display on bills
- `UPIQRCode string` - Base64 encoded UPI QR code image

### 2. Order Model (`models.go`)
**Added Fields:**
- `AmountReceived float64` - Cash amount received from customer
- `ChangeReturned float64` - Change returned to customer

### 3. OrderItem Model (`models.go`)
**Added Fields:**
- `SubId string` - Batch tracking ID for incremental order quantities

## Database Migration

**Migration File:** `internal/migrations/add_profile_and_payment_fields.go`

**Run Migration:**
```sql
-- Add to restaurants table
ALTER TABLE restaurants 
ADD COLUMN IF NOT EXISTS contact_number VARCHAR(50),
ADD COLUMN IF NOT EXISTS upi_qr_code TEXT;

-- Add to orders table
ALTER TABLE orders 
ADD COLUMN IF NOT EXISTS amount_received NUMERIC(10,2),
ADD COLUMN IF NOT EXISTS change_returned NUMERIC(10,2);

-- Add to order_items table with index
ALTER TABLE order_items 
ADD COLUMN IF NOT EXISTS sub_id VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_order_items_sub_id ON order_items(sub_id);
```

## New API Endpoints

### Restaurant Profile Management

#### 1. GET /api/restaurants/profile
**Description:** Get restaurant profile information  
**Authentication:** Required (JWT)  
**Response:**
```json
{
  "id": "uuid",
  "name": "My Restaurant",
  "address": "123 Main St, City",
  "phone": "123-456-7890",
  "contact_number": "987-654-3210",
  "email": "restaurant@example.com",
  "upi_qr_code": "data:image/png;base64,...",
  "city": "Mumbai",
  "cuisine": "Indian"
}
```

#### 2. PUT /api/restaurants/profile
**Description:** Update restaurant profile  
**Authentication:** Required (JWT)  
**Request Body:**
```json
{
  "name": "Updated Restaurant Name",
  "address": "Updated Address",
  "contact_number": "999-888-7777",
  "upi_qr_code": "data:image/png;base64,..."
}
```
**Response:**
```json
{
  "message": "Restaurant profile updated successfully",
  "restaurant": { /* full restaurant object */ }
}
```

### Order Payment Completion

#### 3. POST /api/orders/:order_id/complete-payment
**Description:** Complete order with payment details  
**Authentication:** Required (JWT)  
**Request Body (Cash Payment):**
```json
{
  "payment_method": "cash",
  "amount_received": 550.00,
  "change_returned": 50.00
}
```
**Request Body (UPI Payment):**
```json
{
  "payment_method": "upi",
  "amount_received": 0,
  "change_returned": 0
}
```
**Response:**
```json
{
  "message": "Order completed successfully",
  "order": {
    "id": "uuid",
    "order_number": 123,
    "status": "completed",
    "total": 500.00,
    "payment_method": "cash",
    "amount_received": 550.00,
    "change_returned": 50.00,
    "completed_at": "2024-01-15T10:30:00Z"
  }
}
```

## Implementation Files

### New Files
1. `internal/handlers/restaurant_handler.go` - Restaurant profile handler
2. `internal/migrations/add_profile_and_payment_fields.go` - Database migration

### Modified Files
1. `internal/models/models.go` - Added fields to Restaurant, Order, OrderItem
2. `internal/handlers/order_handler.go` - Added CompleteOrderWithPayment handler
3. `internal/services/order_service.go` - Added CompleteOrderWithPayment service method
4. `internal/handlers/routes.go` - Added new routes

## Service Layer Changes

### OrderService (`order_service.go`)

**New Method:** `CompleteOrderWithPayment()`
```go
func (s *OrderService) CompleteOrderWithPayment(
    restaurantID string,
    orderID string,
    paymentMethod string,
    amountReceived float64,
    changeReturned float64
) (*models.Order, error)
```

**Features:**
- Validates order exists
- Updates order status to "completed"
- Records payment method (cash/upi)
- Stores amount received and change for cash payments
- Sets completion timestamp
- Returns updated order with all payment details

## Validation Rules

### Payment Completion
- `payment_method` is required (must be "cash" or "upi")
- `amount_received` is required for cash payments
- For UPI payments, amount_received and change_returned default to 0

### Restaurant Profile
- At least one field must be provided for update
- `upi_qr_code` should be base64 encoded image data
- Fields not provided in update request remain unchanged

## Testing with Postman/Thunder Client

### 1. Get Restaurant Profile
```http
GET http://localhost:8080/api/restaurants/profile
Authorization: Bearer <your-jwt-token>
```

### 2. Update Restaurant Profile
```http
PUT http://localhost:8080/api/restaurants/profile
Authorization: Bearer <your-jwt-token>
Content-Type: application/json

{
  "name": "BillGenie Test Restaurant",
  "address": "123 Main Street, Mumbai, Maharashtra 400001",
  "contact_number": "9876543210",
  "upi_qr_code": "data:image/png;base64,iVBORw0KG..."
}
```

### 3. Complete Order with Cash Payment
```http
POST http://localhost:8080/api/orders/{order_id}/complete-payment
Authorization: Bearer <your-jwt-token>
Content-Type: application/json

{
  "payment_method": "cash",
  "amount_received": 600.00,
  "change_returned": 100.00
}
```

### 4. Complete Order with UPI Payment
```http
POST http://localhost:8080/api/orders/{order_id}/complete-payment
Authorization: Bearer <your-jwt-token>
Content-Type: application/json

{
  "payment_method": "upi"
}
```

## Next Steps

### 1. Run Database Migration
Execute the migration to add new fields:
```bash
# Option 1: Auto-migrate (if using GORM AutoMigrate)
# Fields will be added automatically on next server start

# Option 2: Manual SQL (for production)
psql -U postgres -d billgenie -f internal/migrations/add_profile_and_payment_fields.sql
```

### 2. Test Endpoints
Use Postman to test all 3 new endpoints:
- GET /api/restaurants/profile
- PUT /api/restaurants/profile  
- POST /api/orders/:order_id/complete-payment

### 3. Frontend Integration
Update frontend API service to use these endpoints:
- Replace AsyncStorage in RestaurantProfileScreen with API calls
- Replace AsyncStorage in BillSummaryScreen with API calls
- Add proper error handling and loading states

### 4. Add Item Status Update with SubId (Future Enhancement)
Current item status update endpoint needs enhancement to support SubId:
```go
// TODO: Update UpdateItemStatus to accept optional sub_id
PATCH /api/orders/:order_id/items/:item_id/status
Body: {
  "status": "cooking",
  "sub_id": "batch-123" // Optional
}
```

## Database Schema Changes Summary

| Table | Column | Type | Description |
|-------|--------|------|-------------|
| restaurants | contact_number | VARCHAR(50) | Display contact on bills |
| restaurants | upi_qr_code | TEXT | Base64 QR code image |
| orders | amount_received | NUMERIC(10,2) | Cash received from customer |
| orders | change_returned | NUMERIC(10,2) | Change returned to customer |
| order_items | sub_id | VARCHAR(100) | Batch tracking identifier |

## Status

✅ Models Updated  
✅ Migration Created  
✅ Restaurant Profile Handler Created  
✅ Order Payment Completion Handler Created  
✅ Service Methods Implemented  
✅ Routes Registered  
⏳ Pending: Run migration  
⏳ Pending: Test endpoints  
⏳ Pending: Frontend integration
