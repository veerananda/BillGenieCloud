# Security rotation checklist

## Week 1 P0 (deployed)

Run after deploying the security P0 API changes.

### 1. Generate new secrets

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

### 2. Lock CORS

```bash
fly secrets set CORS_ALLOWED_ORIGINS="https://thebillgenie.com,https://www.thebillgenie.com" -a billgenie-api
```

No `*`. Include any staging origins you actually use.

### 3. Rotate if secrets were ever shared locally

If `scripts/fly-secrets.env` (gitignored) held production values:

- Rotate DigitalOcean Postgres password and update `DATABASE_URL`
- Rotate Upstash Redis password and update `REDIS_URL`
- Rotate `PLATFORM_OPS_API_KEY`

### 4. Verify P0

- [ ] Web login works from production origin
- [ ] Kitchen / orders WebSocket stays live (subprotocol auth)
- [ ] Creating staff requires password (min 8, not equal to login key)
- [ ] Auth spam returns 429 after limit
- [ ] Old app builds can still connect briefly via `?token=` (deprecated)

## Week 2 P1 (token hashing + refresh secret)

Deployed with `feat/security-p1-hardening`:

- Refresh JWTs are signed with `REFRESH_JWT_SECRET` (legacy access-secret signatures still accepted briefly).
- Refresh tokens, session access tokens, password-reset tokens, email-verification tokens, and login-recovery OTPs are stored as SHA-256 hashes (dual-read of plaintext rows during rollout).
- Non-admin API/menu responses omit `cost_price`; menu WebSocket events clear cost.

### Verify P1

- [ ] Login + refresh still works (existing sessions may need one re-login if refresh JWT was access-signed and DB row was plaintext — dual-read covers both).
- [ ] Password reset email link still works once.
- [ ] Staff/kitchen menu payloads do not include `cost_price`.
- [ ] Payment completion logs do not print cash amounts.
