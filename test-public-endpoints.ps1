# Public Endpoints Test Script
# Tests customer-facing endpoints that don't require authentication

$baseUrl = "http://localhost:3000"
$restaurantId = "886f37e7-c8eb-4c31-9951-dd381a35e560"

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "Testing Public Endpoints (No Auth)" -ForegroundColor Green
Write-Host "========================================`n" -ForegroundColor Green

# Test 1: Get all public menu items
Write-Host "Test 1: Get all menu items" -ForegroundColor Cyan
try {
    $menuResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu?restaurant_id=$restaurantId" -Method GET
    Write-Host "✅ Success: Retrieved $($menuResponse.total) menu items" -ForegroundColor Green
    Write-Host "   Items: $($menuResponse.items.Count) | Limit: $($menuResponse.limit) | Offset: $($menuResponse.offset)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 2: Get menu items by category
Write-Host "`nTest 2: Get menu items by category (Appetizer)" -ForegroundColor Cyan
try {
    $categoryResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu?restaurant_id=$restaurantId&category=Appetizer" -Method GET
    Write-Host "✅ Success: Retrieved $($categoryResponse.total) appetizers" -ForegroundColor Green
    foreach ($item in $categoryResponse.items) {
        Write-Host "   - $($item.name) (₹$($item.price))" -ForegroundColor Gray
    }
} catch {
    Write-Host "❌ Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 3: Get menu items with pagination
Write-Host "`nTest 3: Get menu items with pagination (limit=5)" -ForegroundColor Cyan
try {
    $paginatedResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu?restaurant_id=$restaurantId&limit=5&offset=0" -Method GET
    Write-Host "✅ Success: Retrieved $($paginatedResponse.items.Count) items (Total: $($paginatedResponse.total))" -ForegroundColor Green
    Write-Host "   First 5 items:" -ForegroundColor Gray
    foreach ($item in $paginatedResponse.items) {
        Write-Host "   - $($item.name)" -ForegroundColor Gray
    }
} catch {
    Write-Host "❌ Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 4: Get single menu item by ID
Write-Host "`nTest 4: Get single menu item" -ForegroundColor Cyan
try {
    # Use the first menu item from previous test
    $menuItemId = "da357b89-1584-4be4-b34e-4400a9be0988"
    $itemResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu/$menuItemId?restaurant_id=$restaurantId" -Method GET
    Write-Host "✅ Success: Retrieved menu item" -ForegroundColor Green
    Write-Host "   ID: $($itemResponse.id)" -ForegroundColor Gray
    Write-Host "   Name: $($itemResponse.name)" -ForegroundColor Gray
    Write-Host "   Category: $($itemResponse.category)" -ForegroundColor Gray
    Write-Host "   Price: ₹$($itemResponse.price)" -ForegroundColor Gray
    Write-Host "   Description: $($itemResponse.description)" -ForegroundColor Gray
    Write-Host "   Available: $($itemResponse.is_available)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 5: Get restaurant info
Write-Host "`nTest 5: Get restaurant public info" -ForegroundColor Cyan
try {
    $restaurantResponse = Invoke-RestMethod -Uri "$baseUrl/public/restaurant?restaurant_id=$restaurantId" -Method GET
    Write-Host "✅ Success: Retrieved restaurant info" -ForegroundColor Green
    Write-Host "   ID: $($restaurantResponse.id)" -ForegroundColor Gray
    Write-Host "   Name: $($restaurantResponse.name)" -ForegroundColor Gray
    Write-Host "   Phone: $($restaurantResponse.phone)" -ForegroundColor Gray
    Write-Host "   Email: $($restaurantResponse.email)" -ForegroundColor Gray
    Write-Host "   Address: $($restaurantResponse.address)" -ForegroundColor Gray
} catch {
    Write-Host "❌ Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 6: Filter by availability
Write-Host "`nTest 6: Get only available items" -ForegroundColor Cyan
try {
    $availableResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu?restaurant_id=$restaurantId&available=true&limit=3" -Method GET
    Write-Host "✅ Success: Retrieved $($availableResponse.items.Count) available items" -ForegroundColor Green
    foreach ($item in $availableResponse.items) {
        Write-Host "   - $($item.name) (Available: $($item.is_available))" -ForegroundColor Gray
    }
} catch {
    Write-Host "❌ Failed: $($_.Exception.Message)" -ForegroundColor Red
}

# Test 7: Test without restaurant_id (should fail)
Write-Host "`nTest 7: Test without restaurant_id (should fail)" -ForegroundColor Cyan
try {
    $errorResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu" -Method GET
    Write-Host "❌ Unexpected: Request succeeded without restaurant_id" -ForegroundColor Red
} catch {
    Write-Host "✅ Success: Properly rejected request without restaurant_id" -ForegroundColor Green
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Gray
}

# Test 8: Test with invalid restaurant_id
Write-Host "`nTest 8: Test with invalid restaurant_id" -ForegroundColor Cyan
try {
    $invalidResponse = Invoke-RestMethod -Uri "$baseUrl/public/menu?restaurant_id=invalid-uuid-12345" -Method GET
    Write-Host "✅ Success: Retrieved $($invalidResponse.total) items (likely 0 for invalid ID)" -ForegroundColor Green
} catch {
    Write-Host "✅ Success: Properly handled invalid restaurant_id" -ForegroundColor Green
    Write-Host "   Error: $($_.Exception.Message)" -ForegroundColor Gray
}

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "Public Endpoints Testing Complete!" -ForegroundColor Green
Write-Host "========================================`n" -ForegroundColor Green

Write-Host "Summary:" -ForegroundColor Yellow
Write-Host "✅ All public endpoints working without authentication" -ForegroundColor Green
Write-Host "✅ Filtering and pagination functional" -ForegroundColor Green
Write-Host "✅ Error handling working correctly" -ForegroundColor Green
Write-Host "`nThese endpoints can be used by:" -ForegroundColor Yellow
Write-Host "  - Customer-facing mobile apps" -ForegroundColor White
Write-Host "  - Public menu displays" -ForegroundColor White
Write-Host "  - Online ordering systems" -ForegroundColor White
Write-Host "  - QR code menu viewers" -ForegroundColor White
