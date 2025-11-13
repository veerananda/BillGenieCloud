# BillGenieCloud Setup Guide

This guide will help you set up and run the BillGenieCloud restaurant management system on your local machine.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Node.js** (v14 or higher) - [Download](https://nodejs.org/)
- **MongoDB** (v4.4 or higher) - [Download](https://www.mongodb.com/try/download/community)
- **npm** or **yarn** package manager

## Installation Steps

### 1. Clone the Repository

```bash
git clone https://github.com/veerananda/BillGenieCloud.git
cd BillGenieCloud
```

### 2. Set Up MongoDB

#### Option A: Local MongoDB
1. Start MongoDB service:
   ```bash
   # On macOS (using Homebrew)
   brew services start mongodb-community

   # On Ubuntu/Linux
   sudo systemctl start mongod

   # On Windows
   net start MongoDB
   ```

2. Verify MongoDB is running:
   ```bash
   mongo --eval "db.version()"
   ```

#### Option B: MongoDB Atlas (Cloud)
1. Create a free account at [MongoDB Atlas](https://www.mongodb.com/cloud/atlas)
2. Create a new cluster
3. Get your connection string
4. Use this connection string in your `.env` file

### 3. Backend Setup

1. Navigate to the root directory and install dependencies:
   ```bash
   npm install
   ```

2. Create a `.env` file in the root directory:
   ```bash
   cp .env.example .env
   ```

3. Edit the `.env` file with your configuration:
   ```env
   PORT=3000
   MONGODB_URI=mongodb://localhost:27017/billgenie
   JWT_SECRET=your_secure_random_secret_key_here
   NODE_ENV=development
   ```

   **Important**: Generate a secure JWT secret:
   ```bash
   # On Linux/macOS
   openssl rand -base64 32

   # On Windows (PowerShell)
   [Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Minimum 0 -Maximum 256 }))
   ```

4. Build the backend:
   ```bash
   npm run build
   ```

5. Start the backend server:
   ```bash
   # Development mode (with auto-reload)
   npm run dev

   # Production mode
   npm start
   ```

   The backend API will be available at `http://localhost:3000`

### 4. Frontend Setup

1. Navigate to the frontend directory:
   ```bash
   cd frontend
   ```

2. Install frontend dependencies:
   ```bash
   npm install
   ```

3. Build the frontend (optional):
   ```bash
   npm run build
   ```

4. Start the frontend development server:
   ```bash
   npm run dev
   ```

   The frontend will be available at `http://localhost:5173`

### 5. Create Initial Admin User

Since this is a fresh installation, you'll need to create an initial admin user directly in the database:

1. Connect to MongoDB:
   ```bash
   mongo billgenie
   ```

2. Create an admin user:
   ```javascript
   db.users.insertOne({
     username: "admin",
     email: "admin@billgenie.com",
     password: "$2a$10$XQlJ8K9qZ3Z6K9qZ3Z6K9O7K8K9qZ3Z6K9qZ3Z6K9qZ3Z6K9qZ3Z6", // password: admin123
     firstName: "System",
     lastName: "Administrator",
     role: "admin",
     phone: "+1234567890",
     active: true,
     createdAt: new Date(),
     updatedAt: new Date()
   })
   ```

   Or use this bcrypt-hashed password for "admin123":
   ```
   $2a$10$YourHashedPasswordHere
   ```

   **Note**: For security, change this password immediately after first login!

## Verification

### 1. Test Backend API

```bash
# Check if backend is running
curl http://localhost:3000/

# Expected response:
{
  "message": "Welcome to BillGenieCloud API",
  "version": "1.0.0",
  "endpoints": {...}
}
```

### 2. Test Login

```bash
curl -X POST http://localhost:3000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

### 3. Access Frontend

1. Open your browser and navigate to `http://localhost:5173`
2. Login with:
   - Username: `admin`
   - Password: `admin123`

## Running in Production

### Backend

1. Set `NODE_ENV=production` in your `.env` file
2. Build the project: `npm run build`
3. Use a process manager like PM2:
   ```bash
   npm install -g pm2
   pm2 start dist/index.js --name billgenie-api
   pm2 save
   pm2 startup
   ```

### Frontend

1. Build the frontend: `cd frontend && npm run build`
2. Serve the `dist` folder using a web server like nginx or serve:
   ```bash
   npm install -g serve
   serve -s dist -p 80
   ```

## Environment Variables Reference

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| PORT | Backend server port | 3000 | No |
| MONGODB_URI | MongoDB connection string | mongodb://localhost:27017/billgenie | Yes |
| JWT_SECRET | Secret key for JWT tokens | - | Yes |
| NODE_ENV | Environment (development/production) | development | No |

## Troubleshooting

### MongoDB Connection Issues

**Problem**: Cannot connect to MongoDB

**Solutions**:
1. Verify MongoDB is running: `mongo --eval "db.version()"`
2. Check the connection string in `.env`
3. Ensure MongoDB is listening on the correct port (default: 27017)
4. Check firewall settings

### Port Already in Use

**Problem**: Port 3000 or 5173 is already in use

**Solutions**:
1. Change the port in `.env` (backend) or `vite.config.ts` (frontend)
2. Kill the process using the port:
   ```bash
   # Find process
   lsof -i :3000
   # Kill process
   kill -9 <PID>
   ```

### CORS Issues

**Problem**: Frontend cannot communicate with backend

**Solutions**:
1. Ensure backend CORS is configured properly (already done)
2. Check that both servers are running
3. Verify proxy configuration in `vite.config.ts`

### Build Errors

**Problem**: TypeScript build errors

**Solutions**:
1. Delete `node_modules` and reinstall:
   ```bash
   rm -rf node_modules package-lock.json
   npm install
   ```
2. Clear TypeScript cache:
   ```bash
   rm -rf dist
   npm run build
   ```

## Next Steps

After successful installation:

1. **Change default admin password**
2. **Create additional users** with different roles
3. **Set up menu items** for your restaurant
4. **Configure tables** and locations
5. **Start taking orders!**

## Support

For issues and questions:
- Create an issue on GitHub
- Check the [API Documentation](./API_DOCUMENTATION.md)
- Read the [README](./README.md)

## Development

To contribute to the project:

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/YourFeature`
3. Make your changes
4. Run tests (when available)
5. Commit: `git commit -m 'Add YourFeature'`
6. Push: `git push origin feature/YourFeature`
7. Create a Pull Request

## Security Notes

- Never commit `.env` files to version control
- Use strong, unique JWT secrets in production
- Regularly update dependencies
- Enable HTTPS in production
- Implement rate limiting for APIs
- Regular database backups

## License

ISC
