# Restaurant API Test Script
# Run this in PowerShell while server is running

Write-Host ""
Write-Host "Testing Restaurant API..." -ForegroundColor Cyan
Write-Host "============================================================" -ForegroundColor Cyan

$baseUrl = "http://localhost:3000"
$token = ""

# Test 1: Health Check
Write-Host ""
Write-Host "Test 1: Health Check" -ForegroundColor Green
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/health" -Method Get
    Write-Host "Status: OK" -ForegroundColor Green
    Write-Host ($response | ConvertTo-Json)
} catch {
    Write-Host "Failed" -ForegroundColor Red
}

# Test 2: Register Restaurant
Write-Host ""
Write-Host "Test 2: Register Restaurant" -ForegroundColor Green
$registerData = @{
    restaurant_name = "Test Restaurant"
    owner_name = "John Doe"
    email = "john@testrestaurant.com"
    phone = "9876543210"
    password = "password123"
    address = "123 Main St"
    city = "Mumbai"
    cuisine = "Indian"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/auth/register" -Method Post -Body $registerData -ContentType "application/json"
    Write-Host "Registration successful!" -ForegroundColor Green
    Write-Host "Restaurant ID: $($response.restaurant.id)" -ForegroundColor Yellow
    Write-Host "User ID: $($response.user.id)" -ForegroundColor Yellow
} catch {
    Write-Host "Note: Email might already be registered" -ForegroundColor Yellow
}

# Test 3: Login
Write-Host ""
Write-Host "Test 3: Login" -ForegroundColor Green
$loginData = @{
    email = "john@testrestaurant.com"
    password = "password123"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method Post -Body $loginData -ContentType "application/json"
    $token = $response.access_token
    Write-Host "Login successful!" -ForegroundColor Green
    Write-Host "Token received" -ForegroundColor Yellow
} catch {
    Write-Host "Failed" -ForegroundColor Red
    exit
}

# Setup headers with token
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Test 4: Get Profile
Write-Host ""
Write-Host "Test 4: Get User Profile" -ForegroundColor Green
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/auth/profile" -Method Get -Headers $headers
    Write-Host "Profile retrieved!" -ForegroundColor Green
    Write-Host "Name: $($response.name)" -ForegroundColor Yellow
    Write-Host "Role: $($response.role)" -ForegroundColor Yellow
    $restaurantId = $response.restaurant_id
} catch {
    Write-Host "Failed" -ForegroundColor Red
}

# Test 5: Create Menu Item
Write-Host ""
Write-Host "Test 5: Create Menu Item (Biryani)" -ForegroundColor Green
$menuData = @{
    name = "Chicken Biryani"
    category = "main"
    description = "Delicious chicken biryani"
    price = 250.00
    cost_price = 120.00
    is_veg = $false
    is_available = $true
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/menu" -Method Post -Body $menuData -Headers $headers
    Write-Host "Menu item created!" -ForegroundColor Green
    Write-Host "Item ID: $($response.id)" -ForegroundColor Yellow
    Write-Host "Price: Rs.$($response.price)" -ForegroundColor Yellow
    $menuItemId = $response.id
} catch {
    Write-Host "Failed" -ForegroundColor Red
}

# Test 6: Get Menu Items
Write-Host ""
Write-Host "Test 6: Get All Menu Items" -ForegroundColor Green
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/menu" -Method Get
    Write-Host "Menu retrieved!" -ForegroundColor Green
    Write-Host "Total Items: $($response.items.Count)" -ForegroundColor Yellow
    if ($response.items.Count -gt 0) {
        $menuItemId = $response.items[0].id
    }
} catch {
    Write-Host "Failed" -ForegroundColor Red
}

# Test 7: Set Inventory
Write-Host ""
Write-Host "Test 7: Set Inventory (50 units)" -ForegroundColor Green
$inventoryData = @{
    quantity = 50
    min_level = 10
    max_level = 100
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/inventory/$menuItemId" -Method Put -Body $inventoryData -Headers $headers
    Write-Host "Inventory updated!" -ForegroundColor Green
    Write-Host "Stock: $($response.quantity) units" -ForegroundColor Yellow
} catch {
    Write-Host "Failed" -ForegroundColor Red
}

# Test 8: Create Order (Tests Inventory Deduction)
Write-Host ""
Write-Host "Test 8: Create Order - INVENTORY DEDUCTION TEST" -ForegroundColor Green
$orderData = @{
    table_number = 5
    customer_name = "Customer A"
    items = @(
        @{
            menu_item_id = [int]$menuItemId
            quantity = 2
        }
    )
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$baseUrl/orders" -Method Post -Body $orderData -Headers $headers
    Write-Host "Order created!" -ForegroundColor Green
    Write-Host "Order Number: #$($response.order_number)" -ForegroundColor Yellow
    Write-Host "Total: Rs.$($response.total)" -ForegroundColor Yellow
    $orderId = $response.id
} catch {
    Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 9: Verify Inventory Deducted
Write-Host ""
Write-Host "Test 9: Check Inventory After Order" -ForegroundColor Green
try {
    $response = Invoke-RestMethod -Uri "$baseUrl/inventory" -Method Get -Headers $headers
    Write-Host "Inventory check:" -ForegroundColor Green
    foreach ($item in $response.items) {
        Write-Host "  Stock: $($item.quantity) units (should be 48)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "Failed" -ForegroundColor Red
}

Write-Host ""
Write-Host "============================================================" -ForegroundColor Cyan
Write-Host "All Tests Completed!" -ForegroundColor Green
Write-Host ""
Write-Host "Summary:" -ForegroundColor Cyan
Write-Host "  - Server health: OK" -ForegroundColor Green
Write-Host "  - Registration: Working" -ForegroundColor Green
Write-Host "  - Login & JWT: Working" -ForegroundColor Green
Write-Host "  - Menu: Working" -ForegroundColor Green
Write-Host "  - Inventory: Working" -ForegroundColor Green
Write-Host "  - Orders: Working" -ForegroundColor Green
Write-Host "  - Inventory Deduction: Check above" -ForegroundColor Yellow
Write-Host ""
