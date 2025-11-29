# API Testing Guide - New Endpoints

## Prerequisites
1. Ensure PostgreSQL is running
2. Backend server is running on `http://localhost:8080`
3. You have a valid JWT token from login

## Getting Your JWT Token

First, register and login to get your authentication token:

### Register (if new user)
```bash
POST http://localhost:8080/auth/register
Content-Type: application/json

{
  "restaurant_name": "Test Restaurant",
  "owner_name": "John Doe",
  "email": "test@restaurant.com",
  "password": "password123",
  "phone": "1234567890",
  "city": "Mumbai"
}
```

### Login
```bash
POST http://localhost:8080/auth/login
Content-Type: application/json

{
  "email": "test@restaurant.com",
  "password": "password123"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "uuid",
    "restaurant_id": "restaurant-uuid",
    "name": "John Doe",
    "email": "test@restaurant.com",
    "role": "admin"
  }
}
```

**Copy the `token` value - you'll need it for all API calls below!**

---

## Test Scenario: Complete Restaurant Profile & Order Payment Flow

### Step 1: Get Current Restaurant Profile

```bash
GET http://localhost:8080/api/restaurants/profile
Authorization: Bearer YOUR_JWT_TOKEN_HERE
```

**Expected Response:**
```json
{
  "id": "uuid",
  "name": "Test Restaurant",
  "address": "",
  "phone": "1234567890",
  "contact_number": "",
  "email": "test@restaurant.com",
  "upi_qr_code": "",
  "city": "Mumbai",
  "cuisine": ""
}
```

### Step 2: Update Restaurant Profile

```bash
PUT http://localhost:8080/api/restaurants/profile
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "name": "BillGenie Test Restaurant",
  "address": "123 Main Street, Andheri West, Mumbai, Maharashtra 400058",
  "contact_number": "9876543210",
  "upi_qr_code": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
}
```

**Expected Response:**
```json
{
  "message": "Restaurant profile updated successfully",
  "restaurant": {
    "id": "uuid",
    "name": "BillGenie Test Restaurant",
    "address": "123 Main Street, Andheri West, Mumbai, Maharashtra 400058",
    "phone": "1234567890",
    "contact_number": "9876543210",
    "email": "test@restaurant.com",
    "upi_qr_code": "data:image/png;base64,...",
    "city": "Mumbai"
  }
}
```

### Step 3: Create a Test Order (if you don't have one)

First, create a menu item:

```bash
POST http://localhost:8080/api/menu
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "name": "Chicken Burger",
  "category": "Burgers",
  "description": "Delicious chicken burger",
  "price": 250.00,
  "cost_price": 100.00,
  "is_veg": false
}
```

Then create inventory for it:

```bash
PUT http://localhost:8080/api/inventory/{menu_item_id}
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "quantity": 100,
  "unit": "pieces",
  "low_stock_threshold": 10
}
```

Now create an order:

```bash
POST http://localhost:8080/api/orders
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "table_number": 5,
  "customer_name": "Test Customer",
  "items": [
    {
      "menu_item_id": "YOUR_MENU_ITEM_ID",
      "quantity": 2,
      "notes": "Extra spicy"
    }
  ],
  "notes": "Test order"
}
```

**Note the `order_id` from the response!**

### Step 4: List Orders to Get Order ID

```bash
GET http://localhost:8080/api/orders
Authorization: Bearer YOUR_JWT_TOKEN_HERE
```

### Step 5: Complete Order with Cash Payment

```bash
POST http://localhost:8080/api/orders/{order_id}/complete-payment
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "payment_method": "cash",
  "amount_received": 600.00,
  "change_returned": 100.00
}
```

**Expected Response:**
```json
{
  "message": "Order completed successfully",
  "order": {
    "id": "order-uuid",
    "order_number": 1,
    "status": "completed",
    "total": 500.00,
    "payment_method": "cash",
    "amount_received": 600.00,
    "change_returned": 100.00,
    "completed_at": "2024-01-15T10:30:00Z"
  }
}
```

### Step 6: Complete Another Order with UPI Payment

Create another order (repeat Step 3), then:

```bash
POST http://localhost:8080/api/orders/{order_id_2}/complete-payment
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "payment_method": "upi"
}
```

**Expected Response:**
```json
{
  "message": "Order completed successfully",
  "order": {
    "id": "order-uuid-2",
    "order_number": 2,
    "status": "completed",
    "total": 500.00,
    "payment_method": "upi",
    "amount_received": 0,
    "change_returned": 0,
    "completed_at": "2024-01-15T10:35:00Z"
  }
}
```

---

## Error Test Cases

### Test 1: Invalid Payment Method
```bash
POST http://localhost:8080/api/orders/{order_id}/complete-payment
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "payment_method": "card"
}
```

**Expected:** 400 Bad Request
```json
{
  "error": "payment_method must be 'cash' or 'upi'"
}
```

### Test 2: Missing Amount for Cash Payment
```bash
POST http://localhost:8080/api/orders/{order_id}/complete-payment
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "payment_method": "cash"
}
```

**Expected:** 400 Bad Request
```json
{
  "error": "amount_received is required for cash payments"
}
```

### Test 3: Order Not Found
```bash
POST http://localhost:8080/api/orders/invalid-id/complete-payment
Authorization: Bearer YOUR_JWT_TOKEN_HERE
Content-Type: application/json

{
  "payment_method": "cash",
  "amount_received": 600
}
```

**Expected:** 404 Not Found
```json
{
  "error": "order not found"
}
```

### Test 4: Update Profile Without Authentication
```bash
PUT http://localhost:8080/api/restaurants/profile
Content-Type: application/json

{
  "name": "Unauthorized Update"
}
```

**Expected:** 401 Unauthorized

---

## Postman Collection

If using Postman, import this collection:

1. Create new collection "BillGenie - Restaurant Profile & Payment"
2. Add environment variable `BASE_URL` = `http://localhost:8080`
3. Add environment variable `TOKEN` = `YOUR_JWT_TOKEN`
4. Add requests:

**Collection Structure:**
```
BillGenie - Restaurant Profile & Payment
├── Auth
│   ├── Register
│   └── Login
├── Restaurant Profile
│   ├── Get Profile
│   └── Update Profile
└── Order Payment
    ├── Complete with Cash
    ├── Complete with UPI
    └── Complete (Invalid Method - Error Test)
```

---

## Verification Checklist

After testing, verify:

- [ ] Restaurant profile GET returns all fields
- [ ] Restaurant profile UPDATE saves correctly
- [ ] Updated profile persists after server restart
- [ ] Cash payment completion stores amount_received
- [ ] Cash payment completion stores change_returned
- [ ] UPI payment completion sets amounts to 0
- [ ] Completed orders have completed_at timestamp
- [ ] Order status changes to "completed"
- [ ] Invalid payment methods are rejected
- [ ] Missing cash amount is rejected
- [ ] Non-existent orders return 404
- [ ] Unauthorized requests return 401

---

## Database Verification

Check database directly to confirm changes:

```sql
-- Check restaurant profile fields
SELECT id, name, address, contact_number, 
       LENGTH(upi_qr_code) as qr_length 
FROM restaurants;

-- Check completed orders with payment details
SELECT order_number, status, total, payment_method, 
       amount_received, change_returned, completed_at 
FROM orders 
WHERE status = 'completed';

-- Verify new columns exist
SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'restaurants' 
  AND column_name IN ('contact_number', 'upi_qr_code');

SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'orders' 
  AND column_name IN ('amount_received', 'change_returned');

SELECT column_name, data_type 
FROM information_schema.columns 
WHERE table_name = 'order_items' 
  AND column_name = 'sub_id';
```

---

## Quick Test Script (PowerShell)

Save this as `test-api.ps1`:

```powershell
$BASE_URL = "http://localhost:8080"
$TOKEN = "YOUR_JWT_TOKEN_HERE"

# Test 1: Get Profile
Write-Host "Test 1: Get Restaurant Profile" -ForegroundColor Cyan
$response = Invoke-RestMethod -Uri "$BASE_URL/api/restaurants/profile" `
    -Method GET `
    -Headers @{"Authorization" = "Bearer $TOKEN"}
$response | ConvertTo-Json

# Test 2: Update Profile
Write-Host "`nTest 2: Update Restaurant Profile" -ForegroundColor Cyan
$body = @{
    name = "PowerShell Test Restaurant"
    address = "123 PowerShell Street"
    contact_number = "9999999999"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$BASE_URL/api/restaurants/profile" `
    -Method PUT `
    -Headers @{"Authorization" = "Bearer $TOKEN"; "Content-Type" = "application/json"} `
    -Body $body
$response | ConvertTo-Json

Write-Host "`n✅ All tests completed!" -ForegroundColor Green
```

Run with:
```powershell
.\test-api.ps1
```

---

## Troubleshooting

### Server won't start
```bash
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
go run cmd/server/main.go
```
Check for:
- PostgreSQL running
- .env file configured
- Port 8080 available

### Migration issues
The AutoMigrate should run automatically. Check logs for:
```
🔄 Running database migrations...
✅ Database migrations completed
```

### Cannot connect to API
1. Verify server is running: http://localhost:8080/health
2. Check CORS settings in .env
3. Verify JWT token is valid (not expired)

---

## Next Steps After Testing

1. ✅ Verify all 3 endpoints work
2. Update Postman collection
3. Document any issues
4. Proceed with frontend integration
5. Test WebSocket events for order completion
6. Add integration tests
