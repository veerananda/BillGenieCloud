# Deploy BillGenie API — Fly.io + DigitalOcean Postgres + Upstash Redis

Production stack for India pilot (Rajahmundry):

| Service | Provider | Region | Purpose |
|---------|----------|--------|---------|
| API | Fly.io | `bom` (Mumbai) | HTTP + WebSocket |
| Postgres | DigitalOcean Managed DB | `blr1` (Bangalore) | Primary database |
| Redis | Upstash | `ap-south-1` | WebSocket fan-out across instances |

**Estimated cost:** ~₹1,700–2,800/month at pilot scale.

---

## Prerequisites

- [Fly.io account](https://fly.io) + [Fly CLI](https://fly.io/docs/hands-on/install-flyctl/)
- [DigitalOcean account](https://cloud.digitalocean.com)
- [Upstash account](https://upstash.com)
- PowerShell (Windows) or bash

```powershell
fly auth login
```

---

## Step 1 — DigitalOcean Postgres (Bangalore)

1. **Create → Databases → PostgreSQL**
2. Region: **Bangalore (`blr1`)**
3. Plan: Basic — 1 GB RAM (~$15/mo) is enough for pilot
4. Database name: `billgenie` (or keep `defaultdb`)
5. After creation, open **Connection details** → **Connection string** (URI)
6. Copy the URI — it looks like:
   ```
   postgresql://doadmin:XXXX@db-postgresql-blr1-xxxxx.db.ondigitalocean.com:25060/defaultdb?sslmode=require
   ```
7. **Settings → Trusted sources** → add `0.0.0.0/0` for pilot (or restrict to Fly IPs later)

> Migrations run automatically on first API boot via GORM AutoMigrate.

---

## Step 2 — Upstash Redis

1. **Create database** → Region: **Asia Pacific (Mumbai)** or `ap-south-1`
2. Enable **TLS**
3. Copy the **Redis URL** (`rediss://...`) from the dashboard

---

## Step 3 — Configure Fly secrets

```powershell
cd restaurant-api
copy scripts\fly-secrets.example.env scripts\fly-secrets.env
# Edit fly-secrets.env with your DATABASE_URL, REDIS_URL, JWT secrets
```

Generate JWT secrets (PowerShell):

```powershell
[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
```

Push secrets:

```powershell
.\scripts\set-fly-secrets.ps1
```

Required secrets:

| Secret | Example |
|--------|---------|
| `DATABASE_URL` | DO connection string |
| `REDIS_URL` | Upstash `rediss://...` |
| `JWT_SECRET` | Random 32+ bytes |
| `REFRESH_JWT_SECRET` | Random 32+ bytes |
| `API_BASE_URL` | `https://billgenie-api.fly.dev` |
| `CORS_ALLOWED_ORIGINS` | `*` (pilot) or your app domains |

`SERVER_ENV=production` and `PORT=3000` are set in `fly.toml`.

---

## Step 4 — Deploy to Fly.io

**First time** (creates app):

```powershell
cd restaurant-api
.\scripts\deploy-fly.ps1 -Launch
```

**Subsequent deploys:**

```powershell
.\scripts\deploy-fly.ps1 -SkipSecrets
```

Or manually:

```powershell
fly deploy --app billgenie-api --region bom
```

---

## Step 5 — Verify deployment

```powershell
curl https://billgenie-api.fly.dev/health
```

Expected:

```json
{"status":"ok","service":"restaurant-api","version":"1.0.0"}
```

Check logs (migrations run on boot):

```powershell
fly logs --app billgenie-api
```

Look for:
- `✅ Database connected successfully`
- `✅ Database migrations completed`
- `✅ Redis pub/sub connected` (if REDIS_URL set)

---

## Step 6 — Update mobile app

Edit `BillGenieApp-new/.env.production`:

```env
EXPO_PUBLIC_API_BASE_URL=https://billgenie-api.fly.dev
EXPO_PUBLIC_WS_BASE_URL=wss://billgenie-api.fly.dev
```

For local dev against production API, same URLs in `.env.development`.

Rebuild production APK when ready:

```powershell
cd BillGenieApp-new
eas build --profile production --platform android
```

---

## Step 7 — Register first restaurant

If migrating from an old host, start fresh on DigitalOcean Postgres:

1. Open the app → **Register** a new restaurant
2. Add menu items and tables
3. Test multi-device sync (phone + browser kitchen view)

---

## Troubleshooting

### Database connection failed

- Confirm `DATABASE_URL` includes `sslmode=require`
- Check DO **Trusted sources** allows Fly egress
- `fly secrets list --app billgenie-api`

### WebSocket not connecting

- Use `wss://` (not `ws://`) in production
- Token is passed as query param: `wss://billgenie-api.fly.dev/ws?token=JWT`
- Check CORS / `CORS_ALLOWED_ORIGINS`

### Redis warnings

- `REDIS_URL not set` → local-only WS (fine for 1 machine)
- `Redis ping failed` → verify Upstash URL uses `rediss://` and password is correct

### Build fails on Fly

- Dockerfile uses Go 1.24 (matches `go.mod`)
- No `.env` file needed in image — secrets come from Fly

---

## Useful commands

```powershell
fly status --app billgenie-api
fly logs --app billgenie-api
fly ssh console --app billgenie-api
fly secrets list --app billgenie-api
fly scale count 1 --app billgenie-api
```

---

## Custom domain (optional)

```powershell
fly certs add api.yourdomain.com --app billgenie-api
```

Add DNS CNAME → `billgenie-api.fly.dev`, then update `API_BASE_URL` and app env vars.
