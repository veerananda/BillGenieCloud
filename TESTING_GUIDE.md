# 🧪 Backend Testing Guide

## Prerequisites Check

Before testing the backend, ensure you have:

### ✅ Required (You Have)
- [x] Go 1.21+ installed
- [x] Backend compiled successfully (`bin/restaurant-api.exe` exists)
- [x] `.env` file configured

### 📦 Database Options

**Option A: Install Docker Desktop (Recommended)**
1. Download: https://www.docker.com/products/docker-desktop
2. Install Docker Desktop for Windows
3. Start Docker Desktop
4. Run: `docker compose up -d` in the project directory
5. PostgreSQL will be available at `localhost:5432`

**Option B: Install PostgreSQL Locally**
1. Download: https://www.postgresql.org/download/windows/
2. Install PostgreSQL 14+ with default settings
3. Create database: 
   ```sql
   CREATE DATABASE restaurant_db;
   CREATE USER user WITH PASSWORD 'password';
   GRANT ALL PRIVILEGES ON DATABASE restaurant_db TO user;
   ```
4. Update `.env` file with your PostgreSQL credentials

**Option C: Use Cloud PostgreSQL (Fastest for Testing)**
1. Create free account at https://www.elephantsql.com/
2. Create a free "Tiny Turtle" PostgreSQL instance
3. Copy the connection URL
4. Update `.env` file:
   ```
   DATABASE_HOST=<host-from-elephantsql>
   DATABASE_USER=<user-from-elephantsql>
   DATABASE_PASSWORD=<password-from-elephantsql>
   DATABASE_NAME=<database-from-elephantsql>
   DATABASE_PORT=5432
   ```

---

## 🚀 Step 1: Start the Server

Once PostgreSQL is ready:

```powershell
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
.\bin\restaurant-api.exe
```

You should see:
```
✅ Database connected successfully
🔄 Running database migrations...
✅ Database migrations completed
✅ Server starting on http://localhost:3000
📡 WebSocket available at ws://localhost:3000/ws
🏥 Health check at http://localhost:3000/health
```

---

## 🧪 Step 2: Test API Endpoints

### Test 1: Health Check ✅

```powershell
curl http://localhost:3000/health
```

Expected response:
```json
{
  "status": "ok",
  "service": "restaurant-api",
  "version": "1.0.0"
}
```

### Test 2: Register a Restaurant 🏪

```powershell
curl -X POST http://localhost:3000/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{
    "restaurant_name": "Test Restaurant",
    "owner_name": "John Doe",
    "email": "john@restaurant.com",
    "phone": "9876543210",
    "password": "password123",
    "address": "123 Main Street",
    "city": "Mumbai",
    "cuisine": "Indian"
  }'
```

Expected response:
```json
{
  "success": true,
  "message": "Restaurant registered successfully",
  "data": {
    "restaurant": {...},
    "user": {...},
    "access_token": "eyJhbGc..."
  }
}
```

**Save the `access_token` - you'll need it for subsequent requests!**

### Test 3: Login 🔐

```powershell
curl -X POST http://localhost:3000/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{
    "email": "john@restaurant.com",
    "password": "password123"
  }'
```

Expected response:
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGc...",
    "expires_in": 3600,
    "token_type": "Bearer"
  }
}
```

### Test 4: Create Menu Items 🍕

```powershell
# Replace YOUR_TOKEN with the access_token from login
$token = "YOUR_TOKEN_HERE"

curl -X POST http://localhost:3000/api/v1/menu `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer $token" `
  -d '{
    "name": "Paneer Tikka",
    "category": "appetizer",
    "description": "Grilled cottage cheese with spices",
    "price": 250,
    "cost_price": 120,
    "is_veg": true,
    "is_available": true
  }'
```

Expected response:
```json
{
  "success": true,
  "data": {
    "id": "uuid-here",
    "name": "Paneer Tikka",
    "price": 250,
    ...
  }
}
```

### Test 5: Setup Inventory 📦

```powershell
# Get the menu_item_id from the previous response
curl -X POST http://localhost:3000/api/v1/inventory `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer $token" `
  -d '{
    "menu_item_id": "YOUR_MENU_ITEM_ID",
    "quantity": 50,
    "unit": "pieces",
    "min_level": 10,
    "max_level": 100
  }'
```

### Test 6: Create an Order 🛒

```powershell
curl -X POST http://localhost:3000/api/v1/orders `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer $token" `
  -d '{
    "table_number": 5,
    "customer_name": "Jane Smith",
    "items": [
      {
        "menu_item_id": "YOUR_MENU_ITEM_ID",
        "quantity": 2,
        "notes": "Extra spicy"
      }
    ],
    "notes": "Birthday celebration"
  }'
```

Expected response:
```json
{
  "success": true,
  "data": {
    "id": "order-uuid",
    "order_number": 1,
    "table_number": 5,
    "status": "pending",
    "sub_total": 500,
    "tax_amount": 25,
    "total": 525,
    "items": [...]
  }
}
```

**✅ Check inventory - it should have been reduced by 2!**

### Test 7: Get Inventory 📊

```powershell
curl -X GET http://localhost:3000/api/v1/inventory `
  -H "Authorization: Bearer $token"
```

Expected: Quantity should be 48 (50 - 2)

### Test 8: Get All Orders 📋

```powershell
curl -X GET http://localhost:3000/api/v1/orders `
  -H "Authorization: Bearer $token"
```

---

## 🔥 Testing Inventory Deduction Flow

This is the critical feature! Let's test it step by step:

1. **Create menu item** → Check response has ID
2. **Setup inventory with 50 units** → Verify quantity = 50
3. **Create order with 3 units** → Check order created
4. **Get inventory again** → Should show 47 units ✅
5. **Create another order with 5 units** → Check order created
6. **Get inventory again** → Should show 42 units ✅

This proves the **inventory deduction is working automatically** when orders are created!

---

## 🎯 Testing Complete Order Flow

```powershell
# 1. Register
$register = curl -X POST http://localhost:3000/api/v1/auth/register ...

# 2. Extract token
$token = "extract-from-response"

# 3. Create 3 menu items
curl -X POST http://localhost:3000/api/v1/menu -H "Authorization: Bearer $token" ...

# 4. Setup inventory for all 3 items
curl -X POST http://localhost:3000/api/v1/inventory -H "Authorization: Bearer $token" ...

# 5. Create order with all 3 items
curl -X POST http://localhost:3000/api/v1/orders -H "Authorization: Bearer $token" ...

# 6. Verify inventory deducted for all items
curl -X GET http://localhost:3000/api/v1/inventory -H "Authorization: Bearer $token"

# 7. Complete the order
curl -X PUT http://localhost:3000/api/v1/orders/{order_id}/complete -H "Authorization: Bearer $token"

# 8. Cancel an order (restores inventory)
curl -X DELETE http://localhost:3000/api/v1/orders/{order_id} -H "Authorization: Bearer $token"
```

---

## 📡 Testing WebSocket Real-Time Sync

For WebSocket testing, you'll need a WebSocket client. Here are options:

### Option A: Using Browser Console

1. Open browser console (F12)
2. Run:
```javascript
const ws = new WebSocket('ws://localhost:3000/ws?restaurant_id=YOUR_RESTAURANT_ID&token=YOUR_TOKEN');

ws.onopen = () => console.log('✅ Connected');
ws.onmessage = (e) => console.log('📨 Message:', JSON.parse(e.data));
ws.onerror = (e) => console.error('❌ Error:', e);

// Send a test message
ws.send(JSON.stringify({
  type: 'order_created',
  data: { order_id: 'test-123' }
}));
```

### Option B: Using Postman

1. Open Postman
2. New → WebSocket Request
3. URL: `ws://localhost:3000/ws?restaurant_id=YOUR_RESTAURANT_ID&token=YOUR_TOKEN`
4. Connect
5. Send JSON messages

### Test Multi-Device Sync

1. Open 2 browser tabs with WebSocket connections
2. Create an order via API in one tab
3. Both WebSocket clients should receive the event instantly! ⚡

---

## 🐛 Troubleshooting

### Server won't start

**Error: "Failed to connect to database"**
- Check PostgreSQL is running: `docker compose ps` or check Windows services
- Verify `.env` credentials match PostgreSQL setup
- Try connection string: `psql -h localhost -U user -d restaurant_db`

**Error: "Port 3000 already in use"**
- Change `SERVER_PORT` in `.env` to 3001 or 8080
- Or stop the process using port 3000

### Database migrations fail

- Drop and recreate database:
```sql
DROP DATABASE restaurant_db;
CREATE DATABASE restaurant_db;
```
- Restart server - migrations will run fresh

### Authentication issues

**Error: "Invalid token"**
- Token might be expired (1 hour expiry)
- Login again to get a fresh token
- Ensure `Bearer ` prefix in Authorization header

---

## ✅ Success Checklist

After testing, you should have:

- [ ] Server starts without errors
- [ ] Database migrations complete
- [ ] Health endpoint returns 200
- [ ] Can register a restaurant
- [ ] Can login and get JWT token
- [ ] Can create menu items (authenticated)
- [ ] Can setup inventory
- [ ] **Can create orders and inventory automatically deducts** ✅
- [ ] Can list orders
- [ ] Can complete/cancel orders
- [ ] WebSocket connects and receives events

---

## 📊 Performance Expectations

- Order creation: < 100ms
- Inventory check: < 50ms
- WebSocket latency: < 10ms
- API response time: < 200ms
- Concurrent orders: 40,000+ req/sec (Go benchmark)

---

## 🚀 Next Steps

Once all tests pass:

1. ✅ Backend is production-ready
2. → Move to Frontend Integration
3. → Connect React Native app to API
4. → Deploy to Heroku/DigitalOcean
5. → Multi-device testing

---

## 📞 Need Help?

If tests fail, check:
1. Server logs in terminal
2. Database connection in `.env`
3. PostgreSQL is running
4. Correct API endpoint URLs
5. Valid JWT token in requests

The most important test: **Order creation → Inventory deduction** must work! 🎯
