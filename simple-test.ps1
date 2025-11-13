# Simple Backend API Test
# Tests all endpoints with the fixed profile bug

$baseUrl = "http://localhost:3000"

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "BillGenie Backend API Test" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# Login to get token
Write-Host "1. Logging in..." -ForegroundColor Yellow
$loginBody = @{
    email = "raj@mumbaidelights.com"
    password = "securepass123"
} | ConvertTo-Json

try {
    $loginResponse = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
    $token = $loginResponse.access_token
    $headers = @{ "Authorization" = "Bearer $token" }
    Write-Host "   SUCCESS - Token received" -ForegroundColor Green
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
    exit
}

# Get Profile (Bug Fix Verification)
Write-Host "`n2. Getting profile (verify restaurant_id)..." -ForegroundColor Yellow
try {
    $profile = Invoke-RestMethod -Uri "$baseUrl/auth/profile" -Method Get -Headers $headers
    Write-Host "   SUCCESS" -ForegroundColor Green
    Write-Host "   User ID: $($profile.user_id)" -ForegroundColor Gray
    Write-Host "   Restaurant ID: $($profile.restaurant_id)" -ForegroundColor Gray
    Write-Host "   Role: $($profile.role)" -ForegroundColor Gray
    $restaurantId = $profile.restaurant_id
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
    exit
}

# Create Menu Item
Write-Host "`n3. Creating menu item..." -ForegroundColor Yellow
$menuBody = @{
    name = "Test Dish $(Get-Random -Maximum 1000)"
    category = "Main Course"
    price = 150.00
    description = "Test item"
} | ConvertTo-Json

try {
    $menuResponse = Invoke-RestMethod -Uri "$baseUrl/menu" -Method Post -Body $menuBody -ContentType "application/json" -Headers $headers
    $menuItemId = $menuResponse.menu_item.id
    Write-Host "   SUCCESS - Item ID: $menuItemId" -ForegroundColor Green
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
    exit
}

# Get All Menu Items
Write-Host "`n4. Getting all menu items..." -ForegroundColor Yellow
try {
    $menuItems = Invoke-RestMethod -Uri "$baseUrl/menu" -Method Get -Headers $headers
    Write-Host "   SUCCESS - Found $($menuItems.Count) items" -ForegroundColor Green
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
}

# Update Inventory
Write-Host "`n5. Setting inventory (100 units)..." -ForegroundColor Yellow
$inventoryBody = @{
    quantity = 100
} | ConvertTo-Json

try {
    $inventory = Invoke-RestMethod -Uri "$baseUrl/inventory/$menuItemId" -Method Put -Body $inventoryBody -ContentType "application/json" -Headers $headers
    Write-Host "   SUCCESS - Quantity: $($inventory.quantity)" -ForegroundColor Green
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
}

# Get Inventory
Write-Host "`n6. Getting inventory..." -ForegroundColor Yellow
try {
    $inventoryList = Invoke-RestMethod -Uri "$baseUrl/inventory" -Method Get -Headers $headers
    Write-Host "   SUCCESS - Found $($inventoryList.Count) items" -ForegroundColor Green
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
}

# Create Order (Test Inventory Deduction)
Write-Host "`n7. Creating order (INVENTORY DEDUCTION TEST)..." -ForegroundColor Yellow
$orderBody = @{
    table_number = 5
    items = @(
        @{
            menu_item_id = $menuItemId
            quantity = 3
        }
    )
} | ConvertTo-Json -Depth 10

try {
    $order = Invoke-RestMethod -Uri "$baseUrl/orders" -Method Post -Body $orderBody -ContentType "application/json" -Headers $headers
    Write-Host "   SUCCESS - Order ID: $($order.id)" -ForegroundColor Green
    Write-Host "   Total: Rs.$($order.total_amount)" -ForegroundColor Gray
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   Error: $($_.ErrorDetails.Message)" -ForegroundColor Red
}

# Verify Inventory Deduction
Write-Host "`n8. Verifying inventory deduction..." -ForegroundColor Yellow
try {
    $updatedInventory = Invoke-RestMethod -Uri "$baseUrl/inventory" -Method Get -Headers $headers
    $item = $updatedInventory | Where-Object { $_.menu_item_id -eq $menuItemId }
    if ($item) {
        Write-Host "   SUCCESS - Inventory updated!" -ForegroundColor Green
        Write-Host "   Previous: 100 units" -ForegroundColor Gray
        Write-Host "   Current: $($item.quantity) units" -ForegroundColor Gray
        Write-Host "   Deducted: $(100 - $item.quantity) units" -ForegroundColor Gray
        if ($item.quantity -eq 97) {
            Write-Host "   INVENTORY DEDUCTION WORKING CORRECTLY!" -ForegroundColor Green
        }
    }
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
}

# Get All Orders
Write-Host "`n9. Getting all orders..." -ForegroundColor Yellow
try {
    $orders = Invoke-RestMethod -Uri "$baseUrl/orders" -Method Get -Headers $headers
    Write-Host "   SUCCESS - Found $($orders.Count) orders" -ForegroundColor Green
} catch {
    Write-Host "   FAILED - $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Test Complete!" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan
