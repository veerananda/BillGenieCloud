# Deploy BillGenie API to DigitalOcean Droplet

Production API: `https://api.thebillgenie.com`  
Server layout: `/opt/billgenie/app` (git clone) + `/opt/billgenie/.env`

## Postgres on this Droplet (fresh DB)

Prefer a **local Docker Postgres** on the same droplet instead of DigitalOcean Managed Database (cost). Start empty — the API runs GORM AutoMigrate on boot. Do **not** expose port 5432 publicly.

### One-time DB setup

```bash
cd /opt/billgenie/app
git pull origin main

# 1) Credentials (chmod 600)
cp scripts/db.env.example /opt/billgenie/db.env
nano /opt/billgenie/db.env   # set a long random POSTGRES_PASSWORD
chmod 600 /opt/billgenie/db.env

# 2) Shared network (API + Postgres)
docker network create billgenie_net 2>/dev/null || true

# 3) Start Postgres
docker compose -f docker-compose.droplet-db.yml --env-file /opt/billgenie/db.env up -d
docker ps --filter name=billgenie-postgres
docker exec billgenie-postgres pg_isready -U billgenie -d billgenie
```

### Point the API at local Postgres

Edit `/opt/billgenie/.env` — replace the managed `DATABASE_URL` with:

```bash
DATABASE_URL=postgresql://billgenie:YOUR_PASSWORD@billgenie-postgres:5432/billgenie?sslmode=disable
```

**Required:** include `sslmode=disable`. In production the app otherwise appends `sslmode=require`, which fails against plain Docker Postgres.

Then redeploy so the API joins `billgenie_net`:

```bash
cd /opt/billgenie/app
chmod +x scripts/deploy-droplet.sh scripts/backup-droplet-postgres.sh
./scripts/deploy-droplet.sh
```

### Verify

```bash
curl -fsS https://api.thebillgenie.com/health
docker logs billgenie-api 2>&1 | grep -iE 'Database connected|migrations|Failed to connect'
```

Register a test restaurant or use platform ops against the empty DB. When healthy, **destroy the Managed Database** in the DigitalOcean console to stop billing.

### Backups

```bash
./scripts/backup-droplet-postgres.sh
# files under /opt/billgenie/backups/*.sql.gz
```

Cron (daily 03:15 UTC, keep 14 days via script default):

```bash
crontab -e
# add:
15 3 * * * /opt/billgenie/app/scripts/backup-droplet-postgres.sh >>/var/log/billgenie-pg-backup.log 2>&1
```

Occasionally copy a dump off the droplet (laptop / object storage).

### Firewall

- DO Cloud Firewall / UFW: allow 80/443 (and SSH). **Do not** open 5432 to the world.
- Compose already binds host Postgres to `127.0.0.1:5432` only.

---

## One-time server setup (nginx upstream)

Blue/green deploys need nginx to proxy via named upstream `billgenie_backend`.

### 1. Create initial upstream

```bash
cat >/etc/nginx/conf.d/billgenie-upstream.conf <<'EOF'
upstream billgenie_backend {
    server 127.0.0.1:3000;
    keepalive 32;
}
EOF
```

### 2. Point site config at the upstream

Either replace the site file from the repo (after `git pull`):

```bash
cd /opt/billgenie/app
# Preserve Certbot SSL lines if certbot already edited sites-available/billgenie.
# Safest: edit existing file and change every:
#   proxy_pass http://127.0.0.1:3000...
# to:
#   proxy_pass http://billgenie_backend...
# (for /health use: proxy_pass http://billgenie_backend/health;)
```

Or, if Certbot has not customized the file much, copy the example and re-run certbot:

```bash
cp /opt/billgenie/app/scripts/nginx-billgenie-site.conf /etc/nginx/sites-available/billgenie
nginx -t && systemctl reload nginx
certbot --nginx -d api.thebillgenie.com
```

### 3. Verify

```bash
nginx -t
curl -s https://api.thebillgenie.com/health
```

---

## Manual deploy on the Droplet

```bash
cd /opt/billgenie/app
git pull origin main
chmod +x scripts/deploy-droplet.sh
./scripts/deploy-droplet.sh
```

What it does:

1. Builds `billgenie-api:<git-sha>` and tags `billgenie-api:latest`
2. Ensures Docker network `billgenie_net` exists (for local Postgres)
3. Starts a **new** container on the idle port (`3000` or `3001`) on that network
4. Waits until `http://127.0.0.1:<port>/health` succeeds
5. Points nginx upstream at the new port and reloads
6. Removes the old container and renames the new one to `billgenie-api`

---

## Rollback

List images:

```bash
docker images billgenie-api
```

Roll back to a previous SHA (no rebuild):

```bash
cd /opt/billgenie/app
./scripts/deploy-droplet.sh --image billgenie-api:<previous-sha>
```

---

## GitHub Actions (deploy on Release publish)

Workflow: `.github/workflows/deploy-droplet.yml`

Deploys when you **publish a GitHub Release** (not on merge to `main`).  
You can also run **Actions → Deploy Droplet → Run workflow** and optionally pass a tag/SHA.

### Release flow

1. Merge to `main` as usual (CI still runs; no deploy).
2. GitHub → **Releases** → **Draft a new release**.
3. Choose a tag (e.g. `v1.2.0`) → target `main` → **Publish release**.
4. Workflow checks out that tag on the Droplet and runs `deploy-droplet.sh`.

### Secrets (GitHub repo → Settings → Secrets and variables → Actions)

| Secret | Example | Notes |
|--------|---------|--------|
| `DROPLET_HOST` | `168.144.217.143` | or `api.thebillgenie.com` |
| `DROPLET_USER` | `root` | |
| `DROPLET_SSH_KEY` | full private key PEM | Use a **deploy** key pair, not your laptop key if possible |

### Create a deploy SSH key (on your PC)

```powershell
ssh-keygen -t ed25519 -C "github-deploy-billgenie" -f "$env:USERPROFILE\.ssh\id_ed25519_billgenie_deploy" -N '""'
```

Append the **public** key on the Droplet:

```bash
mkdir -p /root/.ssh
chmod 700 /root/.ssh
echo 'PASTE_PUBLIC_KEY_HERE' >> /root/.ssh/authorized_keys
chmod 600 /root/.ssh/authorized_keys
```

Paste the **private** key file contents into GitHub secret `DROPLET_SSH_KEY` (include `-----BEGIN ... KEY-----` lines).

### Git on the Droplet must be able to pull

If the repo is private, use a [deploy key](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/managing-deploy-keys) (read-only) on `veerananda/BillGenieCloud`, or HTTPS with a fine-scoped PAT stored in the server’s git credential helper.

```bash
cd /opt/billgenie/app
git remote -v
# prefer: git@github.com:veerananda/BillGenieCloud.git
```

---

## Env changes

Edit `/opt/billgenie/.env`, then either:

```bash
./scripts/deploy-droplet.sh --image billgenie-api:latest
```

or full rebuild via `./scripts/deploy-droplet.sh`.

---

## Redis / Upstash

Single Droplet: leave `REDIS_URL` unset.  
Before running two API instances, set a working `rediss://...` Upstash URL in `.env` and redeploy.
