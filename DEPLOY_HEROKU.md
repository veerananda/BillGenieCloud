# Deploy BillGenie Backend to Heroku

## 🚀 Quick Deploy (5 Minutes)

Heroku is the **fastest way** to get your backend live. Perfect for testing and measuring real costs.

### Prerequisites

1. **Heroku Account**: Sign up at https://signup.heroku.com/ (Free)
2. **Heroku CLI**: Download from https://devcenter.heroku.com/articles/heroku-cli
3. **Git**: Already installed (you're using it)
4. **Credit Card**: Required for verification (can stay on free tier)

---

## Step 1: Install Heroku CLI (If Not Installed)

### Windows (PowerShell as Administrator):

```powershell
# Using Scoop
scoop install heroku-cli

# OR Download installer
# Visit: https://devcenter.heroku.com/articles/heroku-cli#install-the-heroku-cli
```

Verify installation:
```powershell
heroku --version
# Should show: heroku/8.x.x
```

---

## Step 2: Login to Heroku

```powershell
heroku login
```

This will open your browser. Login with your Heroku credentials.

---

## Step 3: Prepare Your Repository

### A. Initialize Git (if not already done)

```powershell
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api

# Check if git is initialized
git status

# If not, initialize:
git init
git add .
git commit -m "Initial commit - BillGenie backend"
```

### B. Create .gitignore (Important!)

```powershell
# Create .gitignore
@"
.env
bin/
*.exe
*.log
.DS_Store
"@ | Out-File -FilePath .gitignore -Encoding UTF8
```

### C. Commit .gitignore

```powershell
git add .gitignore
git commit -m "Add .gitignore"
```

---

## Step 4: Create Heroku App

```powershell
# Create app (replace 'billgenie-api' with your preferred name)
heroku create billgenie-api

# This will output something like:
# Creating ⬢ billgenie-api... done
# https://billgenie-api.herokuapp.com/ | https://git.heroku.com/billgenie-api.git
```

**Important**: If the name is taken, try:
- `billgenie-api-prod`
- `billgenie-restaurant-api`
- Or let Heroku generate: `heroku create` (no name)

---

## Step 5: Configure Environment Variables

### Set all required environment variables:

```powershell
# Database (Use your existing Supabase)
heroku config:set DATABASE_URL="postgresql://postgres:BillGenie@123@db.mshyajafowpgnvfpuvss.supabase.co:5432/postgres"
heroku config:set DATABASE_HOST="db.mshyajafowpgnvfpuvss.supabase.co"
heroku config:set DATABASE_USER="postgres"
heroku config:set DATABASE_PASSWORD="BillGenie@123"
heroku config:set DATABASE_NAME="postgres"
heroku config:set DATABASE_PORT="5432"
heroku config:set DATABASE_SSLMODE="require"

# Server
heroku config:set SERVER_PORT="3000"
heroku config:set GIN_MODE="release"

# JWT (Generate a strong secret)
heroku config:set JWT_SECRET="$(openssl rand -base64 32)"
heroku config:set JWT_EXPIRY="24h"
heroku config:set REFRESH_TOKEN_EXPIRY="168h"

# CORS (Update with your frontend URL later)
heroku config:set CORS_ORIGINS="*"

# WebSocket
heroku config:set WS_PING_INTERVAL="30s"
heroku config:set WS_READ_TIMEOUT="60s"
heroku config:set WS_WRITE_TIMEOUT="60s"
```

### Verify configuration:

```powershell
heroku config
```

---

## Step 6: Create Procfile

Heroku needs a `Procfile` to know how to run your app:

```powershell
# Create Procfile
@"
web: bin/restaurant-api
"@ | Out-File -FilePath Procfile -Encoding UTF8 -NoNewline

# Add to git
git add Procfile
git commit -m "Add Procfile for Heroku"
```

---

## Step 7: Update Makefile for Heroku Build

Create a `heroku-build.sh` script:

```powershell
# Create build script
@"
#!/bin/bash
# Build the Go binary
go build -o bin/restaurant-api cmd/server/main.go
"@ | Out-File -FilePath heroku-build.sh -Encoding UTF8

# Commit
git add heroku-build.sh
git commit -m "Add Heroku build script"
```

---

## Step 8: Deploy to Heroku

```powershell
# Push to Heroku
git push heroku main

# OR if your branch is 'master':
git push heroku master
```

### Expected Output:

```
Enumerating objects: 50, done.
Counting objects: 100% (50/50), done.
...
-----> Go app detected
-----> Installing Go 1.23
-----> Running: go build -o bin/restaurant-api cmd/server/main.go
-----> Discovering process types
       Procfile declares types -> web
-----> Compressing...
       Done: 25M
-----> Launching...
       Released v1
       https://billgenie-api.herokuapp.com/ deployed to Heroku
```

---

## Step 9: Scale Your Dyno

```powershell
# Ensure at least one web dyno is running
heroku ps:scale web=1
```

---

## Step 10: Open Your App

```powershell
# Open in browser
heroku open

# OR visit: https://billgenie-api.herokuapp.com/health
```

Test the health endpoint:

```powershell
Invoke-RestMethod -Uri "https://billgenie-api.herokuapp.com/health" -Method Get
```

**Expected Response:**
```json
{
  "status": "ok",
  "timestamp": "2025-11-13T10:30:00Z"
}
```

---

## Step 11: Check Logs

```powershell
# View real-time logs
heroku logs --tail

# View last 100 lines
heroku logs -n 100
```

Look for:
- ✅ "Database migrations completed"
- ✅ "Server listening on :3000"
- ✅ "Auth routes registered"

---

## Step 12: Test All Endpoints

### A. Register a Restaurant

```powershell
$registerBody = @{
    restaurant_name = "Cloud Test Restaurant"
    owner_name = "Heroku Admin"
    email = "admin@cloudtest.com"
    password = "CloudPass123!"
    phone = "+919876543210"
    address = "Cloud Street, Mumbai"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "https://billgenie-api.herokuapp.com/auth/register" -Method Post -Body $registerBody -ContentType "application/json"
$response
```

### B. Login

```powershell
$loginBody = @{
    email = "admin@cloudtest.com"
    password = "CloudPass123!"
} | ConvertTo-Json

$loginResponse = Invoke-RestMethod -Uri "https://billgenie-api.herokuapp.com/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginResponse.access_token
Write-Host "Token: $token" -ForegroundColor Green
```

### C. Get Profile

```powershell
$headers = @{
    "Authorization" = "Bearer $token"
}

Invoke-RestMethod -Uri "https://billgenie-api.herokuapp.com/auth/profile" -Method Get -Headers $headers
```

---

## Step 13: Monitor Costs

### View Current Usage:

```powershell
# Check dyno usage
heroku ps

# Check database (if using Heroku Postgres)
heroku addons

# View metrics (if available)
heroku open metrics
```

### Heroku Pricing Tiers:

| Tier | Cost | Features |
|------|------|----------|
| **Free** (Hobby) | $0/month | - Sleeps after 30min inactivity<br>- 550 dyno hours/month<br>- Good for testing |
| **Basic** | $7/month | - No sleep<br>- 1 web dyno<br>- SSL included |
| **Standard-1X** | $25/month | - 512MB RAM<br>- Metrics<br>- Autoscaling |
| **Standard-2X** | $50/month | - 1GB RAM<br>- Better performance |

### Database Costs:

You're using **Supabase** (external), so no Heroku database costs!

**Total Cost:**
- Free tier: **$0/month** (for testing)
- Basic tier: **$7/month** (₹560/month)
- Standard-1X: **$25/month** (₹2,000/month)

---

## Step 14: Set Up Monitoring

### A. Add UptimeRobot (Free Monitoring)

1. Visit: https://uptimerobot.com/
2. Create account (free)
3. Add monitor:
   - Type: HTTP(S)
   - URL: `https://billgenie-api.herokuapp.com/health`
   - Interval: 5 minutes
4. Get alerts via email/SMS

### B. Enable Heroku Metrics

```powershell
# Open metrics dashboard
heroku open metrics
```

Monitor:
- Response time
- Throughput (requests/min)
- Memory usage
- Error rate

---

## Step 15: Cost Tracking Setup

### Create Cost Tracking Spreadsheet:

| Date | Dyno Cost | Database Cost | Bandwidth | Total | Customers | Cost/Customer |
|------|-----------|---------------|-----------|-------|-----------|---------------|
| Nov 13 | $0 | $0 | $0 | $0 | 1 | $0 |
| Nov 20 | $0 | $0 | $0 | $0 | 5 | $0 |
| Nov 27 | $7 | $0 | $0 | $7 | 10 | $0.70 |

### Set Budget Alerts:

```powershell
# Install Heroku plugins
heroku plugins:install heroku-billing

# Check current usage
heroku billing
```

---

## Step 16: Production Checklist

After deployment, verify:

- [ ] Health endpoint returns 200 OK
- [ ] Registration creates new restaurant
- [ ] Login returns JWT token
- [ ] Profile returns user_id, restaurant_id, role
- [ ] Menu endpoints work (create, list, update)
- [ ] Inventory endpoints work
- [ ] Order creation deducts inventory
- [ ] WebSocket connection works
- [ ] CORS allows frontend origin
- [ ] Logs show no errors
- [ ] Response time < 200ms
- [ ] Server doesn't crash under load

---

## Troubleshooting

### Problem: Build Failed

**Error**: `Failed to compile Go app`

**Solution**:
```powershell
# Check Go version in go.mod
cat go.mod | Select-String "go 1"

# Heroku uses Go 1.23 by default
# If different, update go.mod:
# go 1.23
```

### Problem: App Crashed

**Error**: `State changed from up to crashed`

**Solution**:
```powershell
# Check logs
heroku logs --tail

# Common issues:
# 1. DATABASE_URL not set
heroku config:set DATABASE_URL="postgresql://..."

# 2. Port binding issue (Heroku sets $PORT)
# Update main.go to use os.Getenv("PORT")

# 3. Missing Procfile
# Ensure Procfile exists with: web: bin/restaurant-api
```

### Problem: Dyno Sleeping

**Symptom**: First request takes 10+ seconds

**Solution**:
```powershell
# Upgrade to Basic dyno ($7/month)
heroku ps:type basic
```

### Problem: Database Connection Failed

**Error**: `Failed to connect to database`

**Solution**:
```powershell
# Test Supabase connection
Invoke-RestMethod -Uri "https://db.mshyajafowpgnvfpuvss.supabase.co"

# Verify DATABASE_SSLMODE
heroku config:set DATABASE_SSLMODE="require"

# Check if Supabase allows external connections (it should)
```

---

## Scaling Up

### When You Get More Customers:

**10 Customers:**
- Dyno: Basic ($7/month)
- Database: Supabase Free
- **Total: $7/month** (₹560/month)
- **Cost per customer: ₹56/month**

**50 Customers:**
- Dyno: Standard-1X ($25/month)
- Database: Supabase Pro ($25/month)
- **Total: $50/month** (₹4,000/month)
- **Cost per customer: ₹80/month**

**200 Customers:**
- Dyno: Standard-2X ($50/month) + 1 worker ($50/month)
- Database: Supabase Pro ($25/month)
- **Total: $125/month** (₹10,000/month)
- **Cost per customer: ₹50/month**

---

## Next Steps After Deployment

1. **Week 1: Monitor**
   - Track response times
   - Monitor error rates
   - Measure actual costs

2. **Week 2: Optimize**
   - Add database indexes
   - Implement caching (Redis)
   - Optimize slow queries

3. **Week 3: Scale**
   - Add more dynos if needed
   - Upgrade database if needed
   - Implement CDN for static assets

4. **Week 4: Finalize Pricing**
   - Calculate actual cost per customer
   - Set pricing tiers (₹200-800/month)
   - Start customer outreach

---

## Alternative: Use Makefile

If you set up the Makefile correctly:

```powershell
# One-command deploy
make deploy-heroku
```

This runs all steps automatically.

---

## Support

**Heroku Issues:**
- Docs: https://devcenter.heroku.com/
- Support: https://help.heroku.com/

**BillGenie Issues:**
- Check logs: `heroku logs --tail`
- Restart: `heroku restart`
- Rollback: `heroku rollback`

---

## Summary

✅ **Deployed Backend**: https://billgenie-api.herokuapp.com  
✅ **Health Check**: https://billgenie-api.herokuapp.com/health  
✅ **Cost**: $0-7/month initially  
✅ **Database**: Supabase (already configured)  
✅ **SSL**: Included (HTTPS automatic)  
✅ **Monitoring**: Heroku metrics + UptimeRobot  

**Your API is now LIVE! 🚀**

Time to measure real costs and decide on final pricing.
