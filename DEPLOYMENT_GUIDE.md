# 🚀 Deployment Guide

## Quick Deploy Options

### Option A: Heroku (Easiest - 5 minutes) ⚡

#### Prerequisites
- Heroku account (free tier available)
- Heroku CLI installed: https://devcenter.heroku.com/articles/heroku-cli

#### Steps

```bash
# 1. Login to Heroku
heroku login

# 2. Create a new Heroku app
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
heroku create your-restaurant-api

# 3. Add PostgreSQL database (free tier)
heroku addons:create heroku-postgresql:mini

# 4. Set environment variables
heroku config:set JWT_SECRET=your-production-secret-key-here
heroku config:set CORS_ALLOWED_ORIGINS=https://your-frontend.com
heroku config:set SERVER_ENV=production

# 5. Deploy
git init
git add .
git commit -m "Initial deployment"
git push heroku main

# 6. Check logs
heroku logs --tail

# 7. Open your API
heroku open
```

Your API will be live at: `https://your-restaurant-api.herokuapp.com`

**Database Connection:** Heroku automatically sets `DATABASE_URL` environment variable. The app will use it automatically.

---

### Option B: DigitalOcean App Platform (Recommended for Production) 🌊

#### Prerequisites
- DigitalOcean account
- GitHub repository with your code

#### Steps

1. **Push code to GitHub:**
```bash
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api
git init
git add .
git commit -m "Initial commit"
git remote add origin https://github.com/yourusername/restaurant-api.git
git push -u origin main
```

2. **Create DigitalOcean App:**
- Go to: https://cloud.digitalocean.com/apps
- Click "Create App"
- Connect GitHub repository
- Select your restaurant-api repo
- Configure:
  - **App Type:** Web Service
  - **Build Command:** `go build -o bin/server cmd/server/main.go`
  - **Run Command:** `./bin/server`
  - **Port:** 3000

3. **Add PostgreSQL Database:**
- In DigitalOcean App dashboard
- Click "Add Resource" → "Database"
- Select PostgreSQL
- Plan: $7/month (Production) or $15/month (High availability)
- DigitalOcean automatically sets `DATABASE_URL`

4. **Set Environment Variables:**
```
JWT_SECRET=your-production-secret-key
CORS_ALLOWED_ORIGINS=https://your-frontend.com
SERVER_ENV=production
ENABLE_PAYMENT=true
RAZORPAY_KEY_ID=your-razorpay-key
RAZORPAY_KEY_SECRET=your-razorpay-secret
```

5. **Deploy:**
- Click "Deploy"
- Wait 3-5 minutes
- Your API is live!

**URL:** `https://your-app-name.ondigitalocean.app`

**Cost:** ~$12/month ($5 app + $7 database)

---

### Option C: Railway (Modern Alternative) 🚂

1. Go to: https://railway.app
2. Connect GitHub repo
3. Add PostgreSQL service
4. Deploy automatically on git push
5. Set environment variables in dashboard

**Cost:** $5-10/month

---

## 🔒 Production Checklist

Before deploying to production:

### Security
- [ ] Change JWT_SECRET to strong random string (32+ characters)
- [ ] Update CORS_ALLOWED_ORIGINS to your actual frontend domains
- [ ] Enable HTTPS/SSL (automatic on Heroku/DigitalOcean)
- [ ] Set strong DATABASE_PASSWORD
- [ ] Don't commit .env file to git (add to .gitignore)

### Configuration
- [ ] Set SERVER_ENV=production
- [ ] Configure Razorpay production keys
- [ ] Set up error monitoring (Sentry)
- [ ] Configure log retention
- [ ] Set up database backups

### Testing
- [ ] Test all API endpoints in production
- [ ] Test WebSocket connections
- [ ] Test multi-device sync
- [ ] Load test with 100+ concurrent users
- [ ] Test payment flow end-to-end

---

## 📊 Monitoring & Logging

### Heroku Logging
```bash
# View live logs
heroku logs --tail

# View last 100 lines
heroku logs -n 100

# Filter errors only
heroku logs --tail | grep ERROR
```

### DigitalOcean Logging
- Built-in dashboard at: Apps → Your App → Runtime Logs
- Real-time log streaming
- 7-day retention

### Add Sentry for Error Tracking
```bash
# Install Sentry Go SDK
go get github.com/getsentry/sentry-go

# Add to main.go
import "github.com/getsentry/sentry-go"

sentry.Init(sentry.ClientOptions{
    Dsn: os.Getenv("SENTRY_DSN"),
    Environment: "production",
})
```

---

## 💾 Database Backups

### Heroku Postgres
```bash
# Manual backup
heroku pg:backups:capture

# Download backup
heroku pg:backups:download

# Schedule automatic backups
heroku pg:backups:schedule --at "02:00 UTC"
```

### DigitalOcean Postgres
- Automatic daily backups (included)
- Point-in-time recovery
- Accessible via dashboard

---

## 🔄 CI/CD Pipeline

### GitHub Actions (Free)

Create `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Production

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      
      - name: Run tests
        run: go test ./...
      
      - name: Build
        run: go build -o bin/server cmd/server/main.go
      
      - name: Deploy to Heroku
        uses: akhileshns/heroku-deploy@v3.12.12
        with:
          heroku_api_key: ${{secrets.HEROKU_API_KEY}}
          heroku_app_name: "your-restaurant-api"
          heroku_email: "your-email@example.com"
```

---

## 🌍 Custom Domain Setup

### Heroku
```bash
# Add custom domain
heroku domains:add api.yourrestaurant.com

# Get DNS target
heroku domains

# Add CNAME record in your DNS:
# CNAME: api.yourrestaurant.com → your-app.herokuapp.com
```

### DigitalOcean
1. Go to Settings → Domains
2. Add custom domain
3. Follow DNS configuration instructions
4. SSL automatically provisioned

---

## 📈 Scaling

### Heroku Scaling
```bash
# Scale to 2 web dynos
heroku ps:scale web=2

# Upgrade database
heroku addons:upgrade heroku-postgresql:standard-0
```

### DigitalOcean Scaling
- Dashboard → Components → Edit
- Increase instance size: Basic → Professional
- Add more containers (horizontal scaling)

---

## 🧪 Testing Production

```bash
# Replace with your production URL
$API_URL = "https://your-restaurant-api.herokuapp.com"

# Test health
curl $API_URL/health

# Test registration
curl -X POST $API_URL/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"restaurant_name":"Test","owner_name":"John","email":"test@test.com","phone":"1234567890","password":"test123"}'

# Test WebSocket
# Use browser console or Postman
const ws = new WebSocket('wss://your-restaurant-api.herokuapp.com/ws?restaurant_id=xxx&token=yyy');
```

---

## 💰 Cost Breakdown

### Heroku (Entry Level)
- Eco Dynos: $5/month
- Mini Postgres: $5/month
- **Total: $10/month**
- Scales to ~100 concurrent users

### DigitalOcean (Production)
- Basic App: $5/month
- PostgreSQL: $15/month (managed)
- **Total: $20/month**
- Scales to 500+ concurrent users

### Railway (Alternative)
- Starter: $5/month
- PostgreSQL: $5/month
- **Total: $10/month**

---

## 🚨 Troubleshooting

### App won't start
- Check build logs: `heroku logs --tail`
- Verify environment variables are set
- Ensure database URL is correct
- Check Go version matches (1.21+)

### Database connection fails
- Verify DATABASE_URL is set
- Check database is running: `heroku pg:info`
- Test connection with psql

### WebSocket not connecting
- Ensure wss:// (not ws://) in production
- Check CORS_ALLOWED_ORIGINS includes your frontend
- Verify token is valid

---

## ✅ Post-Deployment

After successful deployment:

1. ✅ Update frontend API_BASE_URL to production URL
2. ✅ Test all features end-to-end
3. ✅ Monitor logs for errors
4. ✅ Set up alerts for downtime
5. ✅ Configure database backups
6. ✅ Test multi-device sync in production
7. ✅ Load test with expected traffic

---

## 📱 Connect Frontend

Update your React Native app:

```javascript
// src/config.js
export const API_BASE_URL = __DEV__ 
  ? 'http://localhost:3000' 
  : 'https://your-restaurant-api.herokuapp.com';

export const WS_URL = __DEV__
  ? 'ws://localhost:3000/ws'
  : 'wss://your-restaurant-api.herokuapp.com/ws';
```

---

## 🎯 Production URLs

After deployment, your API will be available at:

**Heroku:** `https://your-app-name.herokuapp.com`
**DigitalOcean:** `https://your-app-name.ondigitalocean.app`
**Railway:** `https://your-app-name.up.railway.app`

**WebSocket:** Replace `https://` with `wss://` + `/ws`

---

## 🔥 Quick Deploy Command

```bash
# Complete Heroku deployment in one command block
heroku login
heroku create your-restaurant-api
heroku addons:create heroku-postgresql:mini
heroku config:set JWT_SECRET=$(openssl rand -hex 32)
git init
git add .
git commit -m "Deploy"
git push heroku main
heroku open
```

Your restaurant API is now live and ready for multi-device sync! 🎉
