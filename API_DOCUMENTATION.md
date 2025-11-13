# BillGenieCloud API Documentation

## Base URL
```
http://localhost:3000/api
```

## Authentication

All protected endpoints require a JWT token in the Authorization header:
```
Authorization: Bearer <your_jwt_token>
```

## Response Format

All responses follow this format:
```json
{
  "success": true|false,
  "data": {}, // or []
  "error": "error message" // only present if success is false
}
```

---

## Authentication Endpoints

### Login
**POST** `/auth/login`

Request:
```json
{
  "username": "admin",
  "password": "password123"
}
```

Response:
```json
{
  "success": true,
  "data": {
    "user": {
      "id": "...",
      "username": "admin",
      "email": "admin@example.com",
      "firstName": "John",
      "lastName": "Doe",
      "role": "admin"
    },
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

### Register User
**POST** `/auth/register` (Admin/Manager only)

Request:
```json
{
  "username": "newuser",
  "email": "user@example.com",
  "password": "securepassword",
  "firstName": "Jane",
  "lastName": "Smith",
  "role": "waiter",
  "phone": "+1234567890"
}
```

---

## Menu Endpoints

### Get All Menu Items
**GET** `/menu`

Query Parameters:
- `category` (optional): Filter by category
- `available` (optional): Filter by availability (true/false)

Response:
```json
{
  "success": true,
  "data": [
    {
      "_id": "...",
      "name": "Margherita Pizza",
      "description": "Classic pizza with tomato sauce and mozzarella",
      "category": "Pizza",
      "price": 12.99,
      "available": true,
      "preparationTime": 15,
      "ingredients": ["dough", "tomato sauce", "mozzarella", "basil"],
      "createdAt": "2024-01-01T00:00:00.000Z",
      "updatedAt": "2024-01-01T00:00:00.000Z"
    }
  ]
}
```

### Create Menu Item
**POST** `/menu` (Admin/Manager only)

Request:
```json
{
  "name": "Margherita Pizza",
  "description": "Classic pizza with tomato sauce and mozzarella",
  "category": "Pizza",
  "price": 12.99,
  "preparationTime": 15,
  "ingredients": ["dough", "tomato sauce", "mozzarella", "basil"],
  "allergens": ["gluten", "dairy"],
  "available": true,
  "nutritionalInfo": {
    "calories": 266,
    "protein": 11,
    "carbs": 33,
    "fat": 10
  }
}
```

### Update Menu Item
**PUT** `/menu/:id` (Admin/Manager only)

---

## Order Endpoints

### Create Order
**POST** `/orders`

Request:
```json
{
  "tableNumber": 5,
  "customerId": "customer_id_here",
  "orderType": "dine-in",
  "items": [
    {
      "menuItem": "menu_item_id",
      "quantity": 2,
      "price": 12.99,
      "specialInstructions": "No onions"
    }
  ],
  "subtotal": 25.98,
  "tax": 2.60,
  "discount": 0,
  "total": 28.58
}
```

Response:
```json
{
  "success": true,
  "data": {
    "_id": "...",
    "orderNumber": "ORD123456789",
    "tableNumber": 5,
    "status": "pending",
    "orderType": "dine-in",
    "items": [...],
    "total": 28.58,
    "paymentStatus": "pending",
    "createdAt": "2024-01-01T00:00:00.000Z"
  }
}
```

### Get All Orders
**GET** `/orders`

Query Parameters:
- `status` (optional): Filter by status (pending, preparing, ready, served, completed, cancelled)
- `orderType` (optional): Filter by type (dine-in, takeaway, delivery)
- `tableNumber` (optional): Filter by table number

### Update Order Status
**PATCH** `/orders/:id/status`

Request:
```json
{
  "status": "preparing"
}
```

### Update Payment Status
**PATCH** `/orders/:id/payment` (Admin/Manager/Cashier only)

Request:
```json
{
  "paymentStatus": "paid",
  "paymentMethod": "credit_card"
}
```

### Cancel Order
**PATCH** `/orders/:id/cancel`

---

## Customer Endpoints

### Create Customer
**POST** `/customers`

Request:
```json
{
  "firstName": "John",
  "lastName": "Doe",
  "email": "john@example.com",
  "phone": "+1234567890",
  "address": {
    "street": "123 Main St",
    "city": "New York",
    "state": "NY",
    "zipCode": "10001"
  },
  "preferences": ["vegetarian"],
  "allergies": ["peanuts"]
}
```

### Get Customer Order History
**GET** `/customers/:id/orders`

---

## Inventory Endpoints

### Get All Inventory Items
**GET** `/inventory`

Query Parameters:
- `category` (optional): Filter by category
- `lowStock` (optional): Show only low stock items (true/false)

### Create Inventory Item
**POST** `/inventory` (Admin/Manager only)

Request:
```json
{
  "itemName": "Tomatoes",
  "category": "Vegetables",
  "quantity": 50,
  "unit": "kg",
  "reorderLevel": 10,
  "supplier": "Fresh Produce Co",
  "costPerUnit": 2.50,
  "expiryDate": "2024-12-31"
}
```

### Restock Inventory
**PATCH** `/inventory/:id/restock` (Admin/Manager only)

Request:
```json
{
  "quantity": 25
}
```

---

## Table Endpoints

### Get All Tables
**GET** `/tables`

Query Parameters:
- `status` (optional): Filter by status (available, occupied, reserved, cleaning)
- `location` (optional): Filter by location

Response:
```json
{
  "success": true,
  "data": [
    {
      "_id": "...",
      "tableNumber": 1,
      "capacity": 4,
      "status": "available",
      "location": "Main Hall",
      "currentOrderId": null
    }
  ]
}
```

### Create Table
**POST** `/tables` (Admin/Manager only)

Request:
```json
{
  "tableNumber": 1,
  "capacity": 4,
  "location": "Main Hall",
  "status": "available"
}
```

### Update Table Status
**PATCH** `/tables/:id/status`

Request:
```json
{
  "status": "occupied",
  "currentOrderId": "order_id_here"
}
```

---

## Reservation Endpoints

### Create Reservation
**POST** `/reservations`

Request:
```json
{
  "customerId": "customer_id",
  "tableId": "table_id",
  "reservationDate": "2024-12-25T19:00:00.000Z",
  "numberOfGuests": 4,
  "specialRequests": "Window seat please"
}
```

### Get All Reservations
**GET** `/reservations`

Query Parameters:
- `status` (optional): Filter by status (pending, confirmed, seated, completed, cancelled)
- `date` (optional): Filter by date (YYYY-MM-DD)

### Update Reservation Status
**PATCH** `/reservations/:id/status`

Request:
```json
{
  "status": "confirmed"
}
```

---

## Analytics Endpoints

### Get Sales Report
**GET** `/analytics/sales` (Admin/Manager only)

Query Parameters:
- `startDate` (optional): Start date for report (ISO format)
- `endDate` (optional): End date for report (ISO format)

Response:
```json
{
  "success": true,
  "data": {
    "totalRevenue": 15420.50,
    "totalOrders": 234,
    "averageOrderValue": 65.90,
    "ordersByType": {
      "dine-in": 150,
      "takeaway": 60,
      "delivery": 24
    }
  }
}
```

### Get Popular Items
**GET** `/analytics/popular-items` (Admin/Manager only)

Query Parameters:
- `limit` (optional): Number of items to return (default: 10)

Response:
```json
{
  "success": true,
  "data": [
    {
      "_id": {...},
      "totalOrders": 45,
      "totalQuantity": 98,
      "totalRevenue": 1270.02
    }
  ]
}
```

### Get Customer Analytics
**GET** `/analytics/customers` (Admin/Manager only)

Response:
```json
{
  "success": true,
  "data": {
    "totalCustomers": 456,
    "topCustomers": [...],
    "averageOrdersPerCustomer": 3.2
  }
}
```

### Get Dashboard Statistics
**GET** `/analytics/dashboard`

Response:
```json
{
  "success": true,
  "data": {
    "todayOrders": 23,
    "todayRevenue": 1450.75,
    "activeOrders": 8,
    "totalCustomers": 456
  }
}
```

---

## Error Codes

- `400` - Bad Request (validation errors)
- `401` - Unauthorized (missing or invalid token)
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `500` - Internal Server Error

## Rate Limiting

Currently not implemented. Consider implementing rate limiting in production.

## Notes

- All dates are in ISO 8601 format
- All prices are in decimal format
- MongoDB ObjectIds are used for references
- Timestamps (createdAt, updatedAt) are automatically managed
