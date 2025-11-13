# Deploy BillGenie Backend to DigitalOcean

## 🌊 Production Deployment (30 Minutes)

DigitalOcean gives you **full control** over your server. Best for scaling to 100+ restaurants.

### Why DigitalOcean?

- ✅ **Fixed Cost**: $20/month for 2GB RAM server
- ✅ **Bangalore Region**: Low latency for Indian customers (40-60ms)
- ✅ **Full Control**: SSH access, install anything
- ✅ **Scalable**: Easy to upgrade to 4GB, 8GB, etc.
- ✅ **Cost Effective**: $20 can handle 50-100 restaurants

---

## Prerequisites

1. **DigitalOcean Account**: Sign up at https://www.digitalocean.com/ (Free $200 credit)
2. **Credit Card**: Required for verification
3. **SSH Client**: Built into Windows 10+ PowerShell
4. **Domain** (Optional): billgenie.in or similar

---

## Step 1: Create DigitalOcean Account

1. Visit: https://www.digitalocean.com/
2. Click "Sign Up"
3. Use this referral link for **$200 credit**: https://m.do.co/c/[referral-code]
4. Verify email and add credit card

---

## Step 2: Create a Droplet (Virtual Server)

### A. Via Web Interface:

1. Login to DigitalOcean
2. Click **"Create" → "Droplets"**
3. Choose settings:

**Image:**
- Distribution: **Ubuntu 22.04 LTS**

**Plan:**
- Basic (Shared CPU)
- **$20/month**: 2GB RAM, 1 vCPU, 50GB SSD, 2TB transfer
  - This can handle 50-100 restaurants easily

**Datacenter Region:**
- **Bangalore - 1** (blr1) - Lowest latency for India
- Alternative: Singapore (sgp1) if Bangalore not available

**Authentication:**
- **SSH Key** (Recommended) OR
- **Password** (Easier for beginners)

**Hostname:**
- `billgenie-api-prod`

**Tags:**
- production, billgenie, api

4. Click **"Create Droplet"**

### B. Via CLI (doctl):

```powershell
# Install doctl (DigitalOcean CLI)
scoop install doctl

# Authenticate
doctl auth init

# Create droplet
doctl compute droplet create billgenie-api-prod `
  --image ubuntu-22-04-x64 `
  --size s-1vcpu-2gb `
  --region blr1 `
  --ssh-keys YOUR_SSH_KEY_ID `
  --tag-names production,billgenie
```

**Wait 1-2 minutes for droplet to boot.**

---

## Step 3: Get Droplet IP Address

### Via Web:
- Dashboard → Droplets → Copy IP address

### Via CLI:
```powershell
doctl compute droplet list
```

**Example Output:**
```
ID          Name                  Public IPv4      Status
123456789   billgenie-api-prod    159.65.145.123   active
```

**Save this IP**: `159.65.145.123` (yours will be different)

---

## Step 4: Connect to Your Droplet

```powershell
# SSH into droplet (replace with your IP)
ssh root@159.65.145.123

# If using password, enter it when prompted
# If using SSH key, it will connect automatically
```

**You should see:**
```
Welcome to Ubuntu 22.04.3 LTS
root@billgenie-api-prod:~#
```

---

## Step 5: Update System and Install Dependencies

```bash
# Update package list
apt update

# Upgrade packages
apt upgrade -y

# Install essential tools
apt install -y curl wget git build-essential

# Install Go 1.23
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

# Add Go to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify Go installation
go version
# Should show: go version go1.23.0 linux/amd64
```

---

## Step 6: Install PostgreSQL Client (Optional)

Only needed if you want to connect to Supabase from the server:

```bash
apt install -y postgresql-client
```

---

## Step 7: Setup Application Directory

```bash
# Create app directory
mkdir -p /opt/billgenie
cd /opt/billgenie

# Create logs directory
mkdir -p /var/log/billgenie
```

---

## Step 8: Upload Your Application

### Option A: Via Git (Recommended)

```bash
# On server:
cd /opt/billgenie
git clone https://github.com/yourusername/billgenie-backend.git .

# Build the app
go build -o bin/restaurant-api cmd/server/main.go
```

### Option B: Upload Binary via SCP (From Your PC)

```powershell
# On your Windows PC:
cd C:\Users\Veerananda\WorkSpace\billGenieCloud\restaurant-api

# Build for Linux
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -o bin/restaurant-api-linux cmd/server/main.go

# Upload to server
scp bin/restaurant-api-linux root@159.65.145.123:/opt/billgenie/restaurant-api

# Upload .env.example
scp .env.example root@159.65.145.123:/opt/billgenie/.env
```

---

## Step 9: Configure Environment Variables

```bash
# On server:
cd /opt/billgenie

# Create .env file
nano .env
```

**Paste this configuration:**

```bash
# Database (Supabase)
DATABASE_URL=postgresql://postgres:BillGenie@123@db.mshyajafowpgnvfpuvss.supabase.co:5432/postgres
DATABASE_HOST=db.mshyajafowpgnvfpuvss.supabase.co
DATABASE_USER=postgres
DATABASE_PASSWORD=BillGenie@123
DATABASE_NAME=postgres
DATABASE_PORT=5432
DATABASE_SSLMODE=require

# Server
SERVER_PORT=3000
GIN_MODE=release

# JWT (Generate new secret for production!)
JWT_SECRET=CHANGE_THIS_TO_RANDOM_STRING_IN_PRODUCTION
JWT_EXPIRY=24h
REFRESH_TOKEN_EXPIRY=168h

# CORS (Update with your frontend domain)
CORS_ORIGINS=*

# WebSocket
WS_PING_INTERVAL=30s
WS_READ_TIMEOUT=60s
WS_WRITE_TIMEOUT=60s
```

**Save**: Press `Ctrl+X`, then `Y`, then `Enter`

**Generate Secure JWT Secret:**

```bash
# Generate random secret
openssl rand -base64 32

# Output: e.g., "dK8xN2pQ7vR5sT9wU1zA3bC4eF6gH8jL0mN"

# Update .env with this secret
nano .env
# Replace JWT_SECRET value
```

---

## Step 10: Make Binary Executable

```bash
chmod +x /opt/billgenie/restaurant-api
```

---

## Step 11: Test the Application

```bash
# Run the app manually (test mode)
cd /opt/billgenie
./restaurant-api

# You should see:
# 🔧 Loading environment variables...
# 📊 Connecting to database...
# ✅ Database migrations completed
# ✅ Auth routes registered
# ✅ Order routes registered
# ...
# [GIN-debug] Listening and serving HTTP on :3000
```

**Test health endpoint** (from another terminal):

```bash
curl http://localhost:3000/health
```

**Expected:**
```json
{"status":"ok","timestamp":"2025-11-13T..."}
```

**If working, press `Ctrl+C` to stop the server.**

---

## Step 12: Create Systemd Service (Run 24/7)

### A. Create Service File:

```bash
nano /etc/systemd/system/billgenie.service
```

**Paste this:**

```ini
[Unit]
Description=BillGenie Restaurant API
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/billgenie
ExecStart=/opt/billgenie/restaurant-api
Restart=always
RestartSec=10
StandardOutput=append:/var/log/billgenie/app.log
StandardError=append:/var/log/billgenie/error.log
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
```

**Save**: `Ctrl+X`, `Y`, `Enter`

### B. Enable and Start Service:

```bash
# Reload systemd
systemctl daemon-reload

# Enable service (start on boot)
systemctl enable billgenie

# Start service
systemctl start billgenie

# Check status
systemctl status billgenie
```

**Expected:**
```
● billgenie.service - BillGenie Restaurant API
     Loaded: loaded
     Active: active (running) since ...
```

### C. View Logs:

```bash
# Real-time logs
tail -f /var/log/billgenie/app.log

# Errors
tail -f /var/log/billgenie/error.log

# Or use journalctl
journalctl -u billgenie -f
```

---

## Step 13: Setup Nginx Reverse Proxy

### Why Nginx?
- ✅ SSL/HTTPS support
- ✅ Serve on port 80/443 (instead of 3000)
- ✅ Load balancing (future)
- ✅ Static file caching

### A. Install Nginx:

```bash
apt install -y nginx
```

### B. Configure Nginx:

```bash
# Create config file
nano /etc/nginx/sites-available/billgenie
```

**Paste this:**

```nginx
server {
    listen 80;
    server_name 159.65.145.123;  # Replace with your IP or domain

    # Logs
    access_log /var/log/nginx/billgenie-access.log;
    error_log /var/log/nginx/billgenie-error.log;

    # Proxy to Go backend
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        
        # WebSocket support
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # Headers
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://localhost:3000/health;
        access_log off;
    }
}
```

**Save**: `Ctrl+X`, `Y`, `Enter`

### C. Enable Site:

```bash
# Create symlink
ln -s /etc/nginx/sites-available/billgenie /etc/nginx/sites-enabled/

# Test config
nginx -t

# Reload Nginx
systemctl reload nginx

# Ensure Nginx starts on boot
systemctl enable nginx
```

---

## Step 14: Setup Firewall (UFW)

```bash
# Enable firewall
ufw --force enable

# Allow SSH (IMPORTANT!)
ufw allow ssh

# Allow HTTP
ufw allow 80/tcp

# Allow HTTPS (for later)
ufw allow 443/tcp

# Check status
ufw status

# Expected:
# Status: active
# To                         Action      From
# --                         ------      ----
# 22/tcp                     ALLOW       Anywhere
# 80/tcp                     ALLOW       Anywhere
# 443/tcp                    ALLOW       Anywhere
```

---

## Step 15: Setup SSL Certificate (HTTPS)

### A. Install Certbot:

```bash
apt install -y certbot python3-certbot-nginx
```

### B. Get SSL Certificate:

**If you have a domain** (e.g., api.billgenie.in):

```bash
# Update Nginx config with domain
nano /etc/nginx/sites-available/billgenie
# Change: server_name 159.65.145.123;
# To: server_name api.billgenie.in;

# Reload Nginx
systemctl reload nginx

# Get certificate
certbot --nginx -d api.billgenie.in

# Follow prompts:
# - Enter email
# - Agree to terms
# - Redirect HTTP to HTTPS: Yes
```

**If you DON'T have a domain yet:**

Skip SSL for now. You can add it later when you get a domain.

### C. Auto-Renewal:

```bash
# Test renewal
certbot renew --dry-run

# Certbot automatically sets up a cron job for renewal
```

---

## Step 16: Test Production Deployment

### From Your PC:

```powershell
# Replace with your droplet IP
$baseUrl = "http://159.65.145.123"

# Health check
Invoke-RestMethod -Uri "$baseUrl/health" -Method Get

# Register restaurant
$registerBody = @{
    restaurant_name = "Production Test"
    owner_name = "DO Admin"
    email = "admin@production.com"
    password = "ProdPass123!"
    phone = "+919876543210"
    address = "Bangalore, India"
} | ConvertTo-Json

$response = Invoke-RestMethod -Uri "$baseUrl/auth/register" -Method Post -Body $registerBody -ContentType "application/json"
$response

# Login
$loginBody = @{
    email = "admin@production.com"
    password = "ProdPass123!"
} | ConvertTo-Json

$loginResponse = Invoke-RestMethod -Uri "$baseUrl/auth/login" -Method Post -Body $loginBody -ContentType "application/json"
$token = $loginResponse.access_token
Write-Host "Token: $token" -ForegroundColor Green

# Get profile
$headers = @{ "Authorization" = "Bearer $token" }
Invoke-RestMethod -Uri "$baseUrl/auth/profile" -Method Get -Headers $headers
```

---

## Step 17: Monitoring and Maintenance

### A. Check Application Status:

```bash
# Service status
systemctl status billgenie

# Logs
tail -f /var/log/billgenie/app.log

# System resources
htop  # Install: apt install htop
```

### B. Monitor Disk Space:

```bash
df -h
# Check /dev/vda1 usage (should be < 80%)
```

### C. Monitor Memory:

```bash
free -h
```

### D. Monitor Network:

```bash
netstat -tulpn | grep :3000
netstat -tulpn | grep :80
```

### E. Restart Services:

```bash
# Restart app
systemctl restart billgenie

# Restart Nginx
systemctl restart nginx

# Reboot server (if needed)
reboot
```

---

## Step 18: Setup Automated Backups

### A. Database Backups (Supabase):

Supabase Pro ($25/month) includes:
- Daily backups (7 days retention)
- Point-in-time recovery

**Free Tier**: Manual backups via dashboard

### B. Server Backups:

```bash
# Create backup script
nano /opt/backup.sh
```

**Paste:**

```bash
#!/bin/bash
BACKUP_DIR="/opt/backups"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# Backup .env
cp /opt/billgenie/.env $BACKUP_DIR/env_$DATE.bak

# Backup binary
cp /opt/billgenie/restaurant-api $BACKUP_DIR/api_$DATE.bak

# Keep only last 7 days
find $BACKUP_DIR -type f -mtime +7 -delete

echo "Backup completed: $DATE"
```

**Make executable and schedule:**

```bash
chmod +x /opt/backup.sh

# Add to crontab (daily at 2 AM)
crontab -e

# Add this line:
0 2 * * * /opt/backup.sh >> /var/log/backup.log 2>&1
```

---

## Step 19: Cost Tracking

### DigitalOcean Costs:

| Component | Cost | Details |
|-----------|------|---------|
| **Droplet** | $20/month | 2GB RAM, 50GB SSD, 2TB transfer |
| **Backups** (Optional) | $4/month | Weekly snapshots |
| **Floating IP** (Optional) | $0 | While attached to droplet |
| **Load Balancer** (Future) | $12/month | For scaling to 200+ customers |

### Supabase Costs:

| Tier | Cost | Limits |
|------|------|--------|
| **Free** | $0 | 500MB DB, 2GB bandwidth |
| **Pro** | $25/month | 8GB DB, 50GB bandwidth |

### Total Cost Scenarios:

**Initial (1-50 customers):**
- Droplet: $20/month
- Supabase: $0 (free tier)
- **Total: $20/month** (₹1,600/month)
- **Cost per customer (50): ₹32/month**

**Growth (50-100 customers):**
- Droplet: $20/month
- Supabase Pro: $25/month
- **Total: $45/month** (₹3,600/month)
- **Cost per customer (100): ₹36/month**

**Scale (100-200 customers):**
- Droplet (upgraded): $40/month (4GB RAM)
- Supabase Pro: $25/month
- **Total: $65/month** (₹5,200/month)
- **Cost per customer (200): ₹26/month**

---

## Step 20: Performance Optimization

### A. Enable GZIP Compression:

```bash
nano /etc/nginx/nginx.conf
```

**Find and uncomment:**

```nginx
gzip on;
gzip_vary on;
gzip_proxied any;
gzip_comp_level 6;
gzip_types text/plain text/css text/xml text/javascript application/json application/javascript application/xml+rss application/rss+xml font/truetype font/opentype application/vnd.ms-fontobject image/svg+xml;
```

**Reload:**

```bash
systemctl reload nginx
```

### B. Optimize Go Binary:

```bash
# Build with optimizations
cd /opt/billgenie
go build -ldflags="-s -w" -o bin/restaurant-api cmd/server/main.go

# Restart service
systemctl restart billgenie
```

### C. Add Database Connection Pooling:

Already configured in GORM. Verify in code:

```go
// internal/config/database.go
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

---

## Step 21: Scaling Plan

### When to Scale Up:

**50+ Customers:**
- Upgrade to 4GB RAM Droplet: $40/month
- Enable Supabase Pro: $25/month

**100+ Customers:**
- Upgrade to 8GB RAM Droplet: $80/month
- Add Load Balancer: $12/month
- Run 2 app instances

**200+ Customers:**
- Multiple Droplets behind Load Balancer
- Dedicated PostgreSQL Droplet
- Redis for caching
- CDN for static assets

**Scaling Commands:**

```bash
# Resize droplet (via web or CLI)
doctl compute droplet-action resize <droplet-id> --size s-2vcpu-4gb

# Run multiple app instances
systemctl start billgenie@1
systemctl start billgenie@2
# (requires systemd template file)
```

---

## Step 22: Security Hardening

### A. Change SSH Port:

```bash
nano /etc/ssh/sshd_config

# Change:
Port 22
# To:
Port 2222

# Restart SSH
systemctl restart sshd

# Update firewall
ufw allow 2222/tcp
ufw delete allow 22/tcp
```

### B. Disable Root Login:

```bash
# Create non-root user
adduser billgenie
usermod -aG sudo billgenie

# Disable root SSH
nano /etc/ssh/sshd_config
# Set:
PermitRootLogin no

# Restart SSH
systemctl restart sshd
```

### C. Install Fail2Ban:

```bash
apt install -y fail2ban

# Start and enable
systemctl start fail2ban
systemctl enable fail2ban
```

### D. Keep System Updated:

```bash
# Enable automatic security updates
apt install -y unattended-upgrades
dpkg-reconfigure -plow unattended-upgrades
```

---

## Troubleshooting

### Problem: Can't Connect to Droplet

**Solution:**
```bash
# Check firewall
ufw status

# Ensure SSH allowed
ufw allow ssh
```

### Problem: App Not Starting

**Solution:**
```bash
# Check service logs
journalctl -u billgenie -n 50

# Check if port 3000 in use
netstat -tulpn | grep :3000

# Restart service
systemctl restart billgenie
```

### Problem: Database Connection Failed

**Solution:**
```bash
# Test Supabase connection
psql "postgresql://postgres:BillGenie@123@db.mshyajafowpgnvfpuvss.supabase.co:5432/postgres"

# Check .env file
cat /opt/billgenie/.env | grep DATABASE
```

### Problem: High CPU Usage

**Solution:**
```bash
# Check what's using CPU
htop

# Optimize queries (add indexes)
# Implement caching
# Upgrade droplet size
```

---

## Summary

✅ **Server**: 2GB DigitalOcean Droplet in Bangalore  
✅ **App**: Running as systemd service  
✅ **Web Server**: Nginx reverse proxy  
✅ **SSL**: Certbot (when domain added)  
✅ **Firewall**: UFW configured  
✅ **Monitoring**: Logs + systemd  
✅ **Backups**: Automated daily  
✅ **Cost**: $20/month (₹1,600/month)  

**Your API is Production Ready! 🚀**

**Public URL**: `http://YOUR_DROPLET_IP/health`

Next: Monitor for 1 week, measure costs, finalize pricing.
