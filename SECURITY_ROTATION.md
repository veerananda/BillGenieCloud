# Security rotation checklist (Week 1 P0)

Run after deploying the security P0 API changes.

## 1. Generate new secrets

```powershell
# Distinct 32+ byte secrets (do not reuse the same value)
[Convert]::ToBase64String((1..48 | ForEach-Object { Get-Random -Max 256 }))
[Convert]::ToBase64String((1..48 | ForEach-Object { Get-Random -Max 256 }))
```

Set on Fly (example):

```bash
fly secrets set JWT_SECRET="..." REFRESH_JWT_SECRET="..." -a billgenie-api
```

After JWT rotation, all users must log in again.

## 2. Lock CORS

```bash
fly secrets set CORS_ALLOWED_ORIGINS="https://thebillgenie.com,https://www.thebillgenie.com" -a billgenie-api
```

No `*`. Include any staging origins you actually use.

## 3. Rotate if secrets were ever shared locally

If `scripts/fly-secrets.env` (gitignored) held production values:

- Rotate DigitalOcean Postgres password and update `DATABASE_URL`
- Rotate Upstash Redis password and update `REDIS_URL`
- Rotate `PLATFORM_OPS_API_KEY`

## 4. Verify

- [ ] Web login works from production origin
- [ ] Kitchen / orders WebSocket stays live (subprotocol auth)
- [ ] Creating staff requires password (min 8, not equal to login key)
- [ ] Auth spam returns 429 after limit
- [ ] Old app builds can still connect briefly via `?token=` (deprecated)
