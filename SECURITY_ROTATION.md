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

Also ensure localhost CORS is off in production (`fly.toml` sets `ENABLE_LOCALHOST_CORS_IN_PROD=false`). Redeploy after changing `fly.toml`, or unset any override:

```bash
fly secrets unset ENABLE_LOCALHOST_CORS_IN_PROD -a billgenie-api
```

(If the secret is unset, the `fly.toml` `[env]` value applies.)

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

## Week 3 P2 (client token storage + public HTML / anti-enumeration)

- Web: access JWT in memory only (restored via httpOnly `bg_refresh` cookie on load); refresh JWT in httpOnly cookie (`SameSite=None; Secure` in production).
- Mobile: access + refresh JWTs in `expo-secure-store` (migrates from AsyncStorage once).
- Public track/bill error HTML escapes messages; forgot-password / login-recovery responses avoid account enumeration.
- Login credential failures return a single generic message; account-state messages only after password verification.

### Verify P2

- [ ] Web login sets `bg_refresh` cookie; refresh works after clearing any legacy `localStorage.refresh_token`
- [ ] Full page reload restores session via cookie refresh (access token is not in sessionStorage)
- [ ] Mobile login stores tokens in SecureStore
- [ ] Forgot-password for unknown email returns the generic success message
- [ ] Wrong password and unknown login number return the same error text
- [ ] `ENABLE_LOCALHOST_CORS_IN_PROD=false` on Fly; CORS allowlist is production web origins only
- [ ] Vercel security headers present (CSP, HSTS, X-Frame-Options)

## Week 4 P3 (platform ops + deps + public token review)

### Platform ops

Prefer per-actor keys (identity bound to secret):

```bash
fly secrets set PLATFORM_OPS_API_KEYS="veera=...,mani=..." -a billgenie-api
```

Optional IP allowlist:

```bash
fly secrets set PLATFORM_OPS_IP_ALLOWLIST="x.x.x.x,y.y.y.y/24" -a billgenie-api
```

Legacy `PLATFORM_OPS_API_KEY` still works; when used alone, `X-Platform-Actor` remains a soft label.

Review ops actions: `GET /platform/audit-logs?restaurant_id=&limit=50`

### Dependency scanning

- Dependabot watches Go modules + GitHub Actions weekly
- CI runs `govulncheck ./...` on every PR

### Public capability tokens (`/t`, `/b`, `/a`)

| Token | TTL | Auth | PII on page |
|-------|-----|------|-------------|
| `/t/:token` tracking | 4h | unauthenticated (128-bit) | restaurant name, ticket/order #, readiness — no phone/email |
| `/b/:token` bill | 1h | unauthenticated (128-bit) | restaurant name/address/contact, order lines, optional customer first name — no phone |
| `/a/:token` assistance | table-bound | unauthenticated (128-bit) | table name, order totals/items when checkout — no email |

Public `/public/restaurant` returns name/address/phone only (email removed in P3).

### Verify P3

- [ ] Multi-key login: key for `mani` cannot claim actor `veera` via header
- [ ] IP allowlist blocks non-listed clients when set
- [ ] `/platform/audit-logs` lists recent `platform_*` actions
- [ ] Public restaurant JSON has no `email`
- [ ] CI govulncheck step is green
