# 🧪 Complete API Test Flow - With Inventory Deduction

## ✅ **Bug Fixed:**
- **Issue:** Profile endpoint wasn't returning `restaurant_id` and `role`
- **Fix:** Updated `auth_handler.go` GetProfile() to return all user context from JWT
- **Status:** Code updated, server needs restart

---

## 🚀 **Step-by-Step Testing Guide**

### **Step 1: Start the Server**

Open PowerShell and run:
```powershell
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
go run cmd/server/main.go
```

Wait for:
```
✅ Server starting on http://localhost:3000
📡 WebSocket available at ws://localhost:3000/ws
🏥 Health check at http://localhost:3000/health
[GIN-debug] Listening and serving HTTP on :3000
```

**Keep this terminal open!** The server must keep running.

---

### **Step 2: Open a NEW PowerShell Window**

Open a **second** PowerShell terminal for testing.

Navigate to the project:
```powershell
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
```

---

### **Step 3: Run the Test Script**

```powershell
.\test-api.ps1
```

This will test:
- ✅ Health check
- ✅ Registration (new restaurant)
- ✅ Login (JWT token)
- ✅ Profile (should now show restaurant_id!)
- ✅ Create menu item
- ✅ Set inventory
- ✅ Create order (**INVENTORY DEDUCTION TEST**)
- ✅ Verify inventory was deducted

---

## 🎯 **What to Look For**

### **Success Indicators:**

1. **Profile Response (FIXED):**
```json
{
  "message": "Profile retrieved successfully",
  "user_id": "some-uuid",
  "restaurant_id": "some-uuid",  ← THIS SHOULD NOW APPEAR!
  "role": "admin"
}
```

2. **Order Creation:**
```json
{
  "order_number": 1,
  "total": 262.50,  // 250 + 5% GST
  "status": "pending"
}
```

3. **Inventory Check:**
- **Before Order:** 50 units
- **After Order (2 items):** 48 units ← **DEDUCTION WORKING!**

---

## 🐛 **If Something Fails:**

### **Error: "restaurant info not found"**
- **Cause:** restaurant_id not in JWT or profile
- **Fix:** Already fixed! Just restart server

### **Error: Port 3000 already in use**
```powershell
# Kill all go processes
taskkill /F /IM go.exe

# Or find and kill the specific process
netstat -ano | findstr :3000
taskkill /PID <PID_NUMBER> /F
```

### **Error: "insufficient inventory"**
```powershell
# Check current inventory
$token = (Invoke-RestMethod -Uri "http://localhost:3000/auth/login" `
  -Method Post -Body '{"email":"raj@mumbaidelights.com","password":"securepass123"}' `
  -ContentType "application/json").access_token

$headers = @{"Authorization"="Bearer $token"}

Invoke-RestMethod -Uri "http://localhost:3000/inventory" `
  -Method Get -Headers $headers | ConvertTo-Json
```

---

## 📊 **Manual Testing (Alternative)**

If the script doesn't work, test manually:

###Human: let's proceed further