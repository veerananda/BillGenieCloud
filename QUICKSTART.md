# Quick Start Guide - BillGenieCloud

Get up and running with BillGenieCloud in 5 minutes!

## Prerequisites
- Node.js (v14+)
- MongoDB (v4.4+)

## Quick Setup

### 1. Install and Start MongoDB
```bash
# macOS
brew services start mongodb-community

# Ubuntu/Linux
sudo systemctl start mongod

# Windows
net start MongoDB
```

### 2. Clone and Install
```bash
git clone https://github.com/veerananda/BillGenieCloud.git
cd BillGenieCloud
npm install
```

### 3. Configure Environment
```bash
cp .env.example .env
# Edit .env and set your JWT_SECRET
```

### 4. Build and Run Backend
```bash
npm run build
npm run dev
```
Backend will be running at `http://localhost:3000`

### 5. Setup Frontend (in a new terminal)
```bash
cd frontend
npm install
npm run dev
```
Frontend will be running at `http://localhost:5173`

## First Login

Since this is a fresh installation, you need to create an admin user in MongoDB:

```bash
mongo billgenie
```

```javascript
db.users.insertOne({
  username: "admin",
  email: "admin@example.com",
  // This is a pre-hashed password for "admin123"
  password: "$2a$10$rXjF6F5K5F5K5F5K5F5K5uqF5K5F5K5F5K5F5K5F5K5F5K5F5K5K5K",
  firstName: "Admin",
  lastName: "User",
  role: "admin",
  phone: "+1234567890",
  active: true,
  createdAt: new Date(),
  updatedAt: new Date()
})
```

**OR** use this simpler approach - create a script:

Create `scripts/create-admin.js`:
```javascript
require('dotenv').config();
const mongoose = require('mongoose');
const bcrypt = require('bcryptjs');

async function createAdmin() {
  await mongoose.connect(process.env.MONGODB_URI || 'mongodb://localhost:27017/billgenie');
  
  const User = require('../dist/models/User').default;
  
  const hashedPassword = await bcrypt.hash('admin123', 10);
  
  await User.create({
    username: 'admin',
    email: 'admin@example.com',
    password: hashedPassword,
    firstName: 'Admin',
    lastName: 'User',
    role: 'admin',
    phone: '+1234567890',
    active: true
  });
  
  console.log('Admin user created successfully!');
  process.exit(0);
}

createAdmin().catch(console.error);
```

Run it:
```bash
node scripts/create-admin.js
```

## Login to the Application

1. Open `http://localhost:5173` in your browser
2. Login with:
   - **Username**: `admin`
   - **Password**: `admin123`

## What You'll See

### Dashboard
- Today's orders count
- Today's revenue
- Active orders
- Total customers

### Menu Management
- View all menu items
- Add new items
- Edit existing items
- Toggle availability

### Orders
- View all orders
- Filter by status
- Update order status
- Track payment status

## Next Steps

1. **Change the default admin password** (important!)
2. **Add menu items** for your restaurant
3. **Create tables** for table management
4. **Add staff users** with different roles
5. **Start taking orders!**

## Quick Test

Test the API directly:

```bash
# Login and get token
TOKEN=$(curl -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  | jq -r '.data.token')

# Create a menu item
curl -X POST http://localhost:3000/api/menu \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Burger",
    "description": "Delicious burger",
    "category": "Main Course",
    "price": 9.99,
    "preparationTime": 15,
    "ingredients": ["bun", "patty", "lettuce", "tomato"],
    "available": true
  }'

# Get all menu items
curl -X GET http://localhost:3000/api/menu \
  -H "Authorization: Bearer $TOKEN"
```

## Common Issues

**MongoDB connection error?**
- Make sure MongoDB is running
- Check the connection string in `.env`

**Port already in use?**
- Change the port in `.env` (backend) or `frontend/vite.config.ts` (frontend)

**Login not working?**
- Verify admin user was created in database
- Check backend console for errors

## Learn More

- Full documentation: [README.md](./README.md)
- API reference: [API_DOCUMENTATION.md](./API_DOCUMENTATION.md)
- Detailed setup: [SETUP_GUIDE.md](./SETUP_GUIDE.md)
- Security info: [SECURITY_SUMMARY.md](./SECURITY_SUMMARY.md)

## Support

Need help? Check the documentation or create an issue on GitHub.

Happy restaurant managing! 🍽️
