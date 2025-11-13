# BillGenieCloud - Complete Restaurant Management System

BillGenieCloud is a comprehensive cloud-based restaurant management solution that handles all aspects of restaurant operations from menu management to billing, inventory, table reservations, and analytics.

## Features

### 🍽️ Menu Management
- Create, update, and delete menu items
- Categorize items with prices and descriptions
- Track preparation time and ingredients
- Manage nutritional information and allergens
- Control item availability

### 📋 Order Management
- Place orders for dine-in, takeaway, or delivery
- Real-time order status tracking (pending → preparing → ready → served → completed)
- Kitchen Display System (KDS) integration
- Special instructions for each item
- Order cancellation and modification

### 💰 Billing & Payments
- Automatic tax calculation
- Discount management
- Multiple payment methods support
- Payment status tracking (pending/paid/refunded)
- Itemized receipt generation

### 📦 Inventory Management
- Track ingredient stock levels
- Low-stock alerts and reorder levels
- Supplier management
- Cost tracking per unit
- Expiry date monitoring

### 🪑 Table & Reservation Management
- Table status tracking (available/occupied/reserved/cleaning)
- Real-time table occupancy
- Reservation system with time slots
- Special request handling
- Table assignment to orders

### 👥 Customer Management
- Customer profiles with contact information
- Order history tracking
- Loyalty points program
- Total spending analytics
- Customer preferences and allergies

### 👨‍💼 Staff Management
- User authentication and authorization
- Role-based access control (Admin, Manager, Waiter, Chef, Cashier)
- Secure password hashing
- JWT-based authentication

### 📊 Reporting & Analytics
- Sales reports with date filtering
- Popular items analysis
- Customer analytics and top customers
- Revenue tracking
- Dashboard with key metrics

## Technology Stack

- **Backend**: Node.js + Express.js + TypeScript
- **Database**: MongoDB with Mongoose ODM
- **Authentication**: JWT (JSON Web Tokens)
- **Security**: bcryptjs for password hashing

## Prerequisites

- Node.js (v14 or higher)
- MongoDB (v4.4 or higher)
- npm or yarn

## Installation

1. Clone the repository:
```bash
git clone https://github.com/veerananda/BillGenieCloud.git
cd BillGenieCloud
```

2. Install dependencies:
```bash
npm install
```

3. Create a `.env` file in the root directory:
```env
PORT=3000
MONGODB_URI=mongodb://localhost:27017/billgenie
JWT_SECRET=your_secure_jwt_secret_key
NODE_ENV=development
```

4. Build the project:
```bash
npm run build
```

5. Start the server:
```bash
# Production
npm start

# Development with auto-reload
npm run dev
```

## API Endpoints

### Authentication
- `POST /api/auth/login` - User login
- `POST /api/auth/register` - Register new user (Admin/Manager only)
- `GET /api/auth/users` - Get all users (Admin/Manager only)
- `GET /api/auth/users/:id` - Get user by ID
- `PUT /api/auth/users/:id` - Update user (Admin/Manager only)
- `DELETE /api/auth/users/:id` - Delete user (Admin only)

### Menu Items
- `GET /api/menu` - Get all menu items
- `GET /api/menu/:id` - Get menu item by ID
- `POST /api/menu` - Create menu item (Admin/Manager only)
- `PUT /api/menu/:id` - Update menu item (Admin/Manager only)
- `DELETE /api/menu/:id` - Delete menu item (Admin/Manager only)

### Orders
- `POST /api/orders` - Create new order
- `GET /api/orders` - Get all orders
- `GET /api/orders/:id` - Get order by ID
- `PATCH /api/orders/:id/status` - Update order status
- `PATCH /api/orders/:id/payment` - Update payment status (Admin/Manager/Cashier)
- `PATCH /api/orders/:id/cancel` - Cancel order

### Customers
- `POST /api/customers` - Create customer
- `GET /api/customers` - Get all customers
- `GET /api/customers/:id` - Get customer by ID
- `GET /api/customers/:id/orders` - Get customer's order history
- `PUT /api/customers/:id` - Update customer
- `DELETE /api/customers/:id` - Delete customer (Admin/Manager only)

### Inventory
- `POST /api/inventory` - Create inventory item (Admin/Manager only)
- `GET /api/inventory` - Get all inventory items
- `GET /api/inventory/:id` - Get inventory item by ID
- `PUT /api/inventory/:id` - Update inventory item (Admin/Manager only)
- `PATCH /api/inventory/:id/restock` - Restock inventory (Admin/Manager only)
- `DELETE /api/inventory/:id` - Delete inventory item (Admin/Manager only)

### Tables
- `POST /api/tables` - Create table (Admin/Manager only)
- `GET /api/tables` - Get all tables
- `GET /api/tables/:id` - Get table by ID
- `PATCH /api/tables/:id/status` - Update table status
- `PUT /api/tables/:id` - Update table (Admin/Manager only)
- `DELETE /api/tables/:id` - Delete table (Admin/Manager only)

### Reservations
- `POST /api/reservations` - Create reservation
- `GET /api/reservations` - Get all reservations
- `GET /api/reservations/:id` - Get reservation by ID
- `PATCH /api/reservations/:id/status` - Update reservation status
- `DELETE /api/reservations/:id` - Delete reservation (Admin/Manager only)

### Analytics
- `GET /api/analytics/sales` - Get sales report (Admin/Manager only)
- `GET /api/analytics/popular-items` - Get popular items (Admin/Manager only)
- `GET /api/analytics/customers` - Get customer analytics (Admin/Manager only)
- `GET /api/analytics/dashboard` - Get dashboard statistics

## Authentication

Most endpoints require authentication. Include the JWT token in the Authorization header:

```
Authorization: Bearer <your_jwt_token>
```

## User Roles

- **Admin**: Full system access
- **Manager**: Most operations except user deletion
- **Waiter**: Order and table management
- **Chef**: View orders and update cooking status
- **Cashier**: Payment processing

## Example Usage

### 1. Login
```bash
curl -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "password123"
  }'
```

### 2. Create Menu Item
```bash
curl -X POST http://localhost:3000/api/menu \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "name": "Margherita Pizza",
    "description": "Classic pizza with tomato sauce and mozzarella",
    "category": "Pizza",
    "price": 12.99,
    "preparationTime": 15,
    "ingredients": ["dough", "tomato sauce", "mozzarella", "basil"],
    "available": true
  }'
```

### 3. Place Order
```bash
curl -X POST http://localhost:3000/api/orders \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "tableNumber": 5,
    "orderType": "dine-in",
    "items": [
      {
        "menuItem": "<menu_item_id>",
        "quantity": 2,
        "price": 12.99
      }
    ],
    "subtotal": 25.98,
    "tax": 2.60,
    "total": 28.58
  }'
```

## Database Models

### MenuItem
- name, description, category
- price, imageUrl, available
- preparationTime, ingredients, allergens
- nutritionalInfo (calories, protein, carbs, fat)

### Order
- orderNumber, tableNumber, customerId
- items (menuItem, quantity, price, specialInstructions)
- status, orderType, subtotal, tax, discount, total
- paymentStatus, paymentMethod

### Customer
- firstName, lastName, email, phone
- address, loyaltyPoints, totalOrders, totalSpent
- preferences, allergies

### Inventory
- itemName, category, quantity, unit
- reorderLevel, supplier, costPerUnit
- lastRestocked, expiryDate

### Table
- tableNumber, capacity, status
- currentOrderId, location

### Reservation
- customerId, tableId, reservationDate
- numberOfGuests, status, specialRequests

### User
- username, email, password (hashed)
- firstName, lastName, role, phone, active

## Development

### Project Structure
```
BillGenieCloud/
├── src/
│   ├── config/
│   │   └── database.ts
│   ├── controllers/
│   │   ├── analyticsController.ts
│   │   ├── authController.ts
│   │   ├── customerController.ts
│   │   ├── inventoryController.ts
│   │   ├── menuController.ts
│   │   ├── orderController.ts
│   │   ├── reservationController.ts
│   │   └── tableController.ts
│   ├── middleware/
│   │   └── auth.ts
│   ├── models/
│   │   ├── Customer.ts
│   │   ├── Inventory.ts
│   │   ├── MenuItem.ts
│   │   ├── Order.ts
│   │   ├── Reservation.ts
│   │   ├── Table.ts
│   │   └── User.ts
│   ├── routes/
│   │   ├── analyticsRoutes.ts
│   │   ├── authRoutes.ts
│   │   ├── customerRoutes.ts
│   │   ├── inventoryRoutes.ts
│   │   ├── menuRoutes.ts
│   │   ├── orderRoutes.ts
│   │   ├── reservationRoutes.ts
│   │   └── tableRoutes.ts
│   └── index.ts
├── dist/
├── .env
├── .gitignore
├── package.json
├── tsconfig.json
└── README.md
```

## Security Features

- Password hashing with bcrypt
- JWT-based authentication
- Role-based access control (RBAC)
- Protected routes with middleware
- Environment variable configuration

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

ISC

## Support

For support, email support@billgeniecloud.com or open an issue in the repository.

## Roadmap

- [ ] Frontend React dashboard
- [ ] Real-time WebSocket notifications
- [ ] Payment gateway integration
- [ ] Multi-restaurant support
- [ ] Mobile app for waiters
- [ ] QR code menu for customers
- [ ] Advanced analytics and reporting
- [ ] Email/SMS notifications
- [ ] Online ordering system
- [ ] Integration with delivery platforms
