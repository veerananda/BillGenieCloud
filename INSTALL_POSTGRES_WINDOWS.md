# 🐘 PostgreSQL Installation Guide for Windows

## Option 1: Download Installer (Easiest - 5 minutes)

### Step 1: Download PostgreSQL
1. Go to: https://www.postgresql.org/download/windows/
2. Click "Download the installer"
3. Select **PostgreSQL 16** (latest version)
4. Download the Windows x86-64 installer

### Step 2: Install PostgreSQL
1. Run the downloaded `.exe` file
2. Click "Next" through the setup wizard
3. **Important settings:**
   - Installation Directory: `C:\Program Files\PostgreSQL\16`
   - Port: **5432** (default - keep this)
   - Password: Set to **`password`** (or remember your password)
   - Locale: Default

4. **Uncheck** "Stack Builder" at the end (not needed)
5. Click "Finish"

### Step 3: Verify Installation
Open a **new** PowerShell window and run:
```powershell
psql --version
```

You should see: `psql (PostgreSQL) 16.x`

### Step 4: Create Database
```powershell
# Connect to PostgreSQL (password: what you set during install)
psql -U postgres

# Inside psql, run these commands:
CREATE DATABASE restaurant_db;
CREATE USER user WITH PASSWORD 'password';
GRANT ALL PRIVILEGES ON DATABASE restaurant_db TO user;
\q
```

---

## Option 2: Use Free Cloud PostgreSQL (No Installation - 2 minutes)

### A. ElephantSQL (Free 20MB)
1. Go to: https://www.elephantsql.com/
2. Sign up (free account)
3. Create new instance (Tiny Turtle - FREE)
4. Copy the connection URL
5. Update `.env` file:
```env
DATABASE_HOST=<hostname from URL>
DATABASE_USER=<user from URL>
DATABASE_PASSWORD=<password from URL>
DATABASE_NAME=<database from URL>
DATABASE_PORT=5432
```

### B. Supabase (Free 500MB + More features)
1. Go to: https://supabase.com/
2. Sign up and create new project
3. Wait 2 minutes for database setup
4. Go to "Settings" → "Database"
5. Copy connection details
6. Update `.env` file with the credentials

### C. Render (Free 256MB)
1. Go to: https://render.com/
2. Sign up (free account)
3. Create "New PostgreSQL" database
4. Free tier: 256MB, 90 days (auto-deleted after 90 days)
5. Copy connection string
6. Update `.env` with credentials

---

## Option 3: Use Docker Desktop (If you want containers)

### Step 1: Install Docker Desktop
1. Download: https://www.docker.com/products/docker-desktop/
2. Install Docker Desktop for Windows
3. Restart computer if prompted
4. Start Docker Desktop

### Step 2: Start PostgreSQL Container
```powershell
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api

# Start PostgreSQL using docker-compose
docker-compose up -d

# Verify it's running
docker-compose ps
```

---

## Next Steps After Installation

### 1. Create .env file
```powershell
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
cp .env.example .env
```

### 2. Edit .env file
**For Local PostgreSQL (Option 1):**
```env
DATABASE_HOST=localhost
DATABASE_USER=postgres
DATABASE_PASSWORD=password
DATABASE_NAME=restaurant_db
DATABASE_PORT=5432
SERVER_PORT=3000
JWT_SECRET=my-super-secret-jwt-key-change-this-in-production
ENVIRONMENT=development
```

**For Cloud PostgreSQL (Option 2):**
Use the credentials from your cloud provider

### 3. Run the Backend
```powershell
# Navigate to project
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api

# Run the server
.\bin\restaurant-api.exe
```

### 4. Test Health Check
Open browser or use curl:
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

---

## Troubleshooting

### PostgreSQL won't start
- Check if port 5432 is already in use:
```powershell
netstat -ano | findstr :5432
```

### "Connection refused" error
- Verify PostgreSQL service is running:
  - Windows: Open "Services" app → Look for "postgresql-x64-16"
  - Should be "Running"

### Password authentication failed
- Check your `.env` file matches your PostgreSQL password
- For local install, default user is `postgres`
- For cloud, use the provided credentials

---

## My Recommendation

**Best for Testing:** Option 2 (Cloud) - Supabase
- Free 500MB
- No installation needed
- Works from anywhere
- Includes pgAdmin-like interface
- Takes only 2 minutes to setup

**Best for Development:** Option 1 (Local Install)
- Full control
- No internet needed
- Fastest performance
- Good for learning PostgreSQL

**Best for Teams:** Option 3 (Docker)
- Consistent across all developers
- Easy to reset/restart
- Includes pgAdmin for database management
