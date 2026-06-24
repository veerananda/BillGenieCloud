# Deployment Guide

> **Use [DEPLOY_FLY.md](./DEPLOY_FLY.md)** for production — Fly.io (Mumbai) + DigitalOcean Postgres (Bangalore) + Upstash Redis.

Heroku is no longer used. Legacy Heroku docs have been removed from this repository.

## Production stack

| Component | Provider | URL / region |
|-----------|----------|--------------|
| API + WebSocket | Fly.io | `https://billgenie-api.fly.dev` (`bom`) |
| PostgreSQL | DigitalOcean Managed DB | `blr1` |
| Redis (WS fan-out) | Upstash | `ap-south-1` |

## Quick deploy

```powershell
cd restaurant-api
copy scripts\fly-secrets.example.env scripts\fly-secrets.env
# Edit fly-secrets.env with DATABASE_URL, REDIS_URL, JWT secrets
.\scripts\set-fly-secrets.ps1
.\scripts\deploy-fly.ps1
```

Or: `make deploy-fly`

## Verify

```powershell
curl https://billgenie-api.fly.dev/health
fly logs -a billgenie-api
```

## Local development

```bash
cp .env.example .env
docker-compose up -d
go run cmd/server/main.go
```

## Secrets

Never commit `scripts/fly-secrets.env` or real `.env` files. Production secrets are set via `fly secrets set` (see `scripts/set-fly-secrets.ps1`).

## Frontend

Point the Expo app at production:

```
EXPO_PUBLIC_API_BASE_URL=https://billgenie-api.fly.dev
EXPO_PUBLIC_WS_BASE_URL=wss://billgenie-api.fly.dev
```

`eas.json` production profile and `.env.example` in BillGenieFrontEnd already use these URLs.
