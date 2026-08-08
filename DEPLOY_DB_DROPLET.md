# Deploy BillGenie Postgres on a dedicated DigitalOcean Droplet

Use a **second droplet** for Postgres (cheaper than Managed DB; keeps DB off the API box).  
API stays on the existing droplet (`api.thebillgenie.com`). Fresh empty database — no data migrate.

```text
[App/Web] → API droplet (nginx + billgenie-api)
                │  VPC private IP :5432
                ▼
           DB droplet (Docker Postgres)
```

## 1. Create the DB droplet (DigitalOcean console)

1. DigitalOcean → **Create** → **Droplets**.
2. **Region:** same as the API droplet (required for private networking).
3. **Image:** Ubuntu 24.04 LTS (or 22.04).
4. **Size:** start small — **1 vCPU / 1 GB RAM** (Basic) is enough pre-go-live; bump if needed.
5. **VPC:** choose the **same VPC** as the API droplet.
6. **Authentication:** same SSH key you use for the API droplet.
7. **Hostname:** e.g. `billgenie-db`.
8. **Tags (optional):** `billgenie`, `db`.
9. Create → note **public IP** (SSH) and **private IP** (for `DATABASE_URL`).

Private IP: Droplet → **Networking** → Private IPv4 (e.g. `10.x.x.x`).

## 2. Firewall (critical)

**Do not** allow Postgres from the public internet.

### Option A — DO Cloud Firewall (preferred)

Create firewall `billgenie-db`:

| Direction | Protocol | Ports | Sources |
|-----------|----------|-------|---------|
| Inbound | TCP | 22 | Your IP / bastion only |
| Inbound | TCP | 5432 | **API droplet** (by droplet or tag) only |
| Outbound | All | All | Allow |

Attach firewall to the DB droplet. Do **not** add `0.0.0.0/0` on 5432.

### Option B — UFW on the DB droplet

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
# Replace with API droplet PRIVATE IP:
ufw allow from API_PRIVATE_IP to any port 5432 proto tcp
ufw enable
ufw status
```

## 3. Install Docker on the DB droplet

SSH in (`ssh root@DB_PUBLIC_IP`):

```bash
apt update && apt install -y ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $VERSION_CODENAME stable" \
  > /etc/apt/sources.list.d/docker.list
apt update && apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
```

## 4. Clone app repo (scripts + compose only)

```bash
mkdir -p /opt/billgenie
git clone git@github.com:veerananda/BillGenieCloud.git /opt/billgenie/app
# or HTTPS with a read-only deploy key / PAT
cd /opt/billgenie/app
```

## 5. Start Postgres

```bash
cp scripts/db.env.example /opt/billgenie/db.env
nano /opt/billgenie/db.env   # strong POSTGRES_PASSWORD
chmod 600 /opt/billgenie/db.env

chmod +x scripts/backup-droplet-postgres.sh
docker compose -f docker-compose.droplet-db.yml --env-file /opt/billgenie/db.env up -d
docker ps --filter name=billgenie-postgres
docker exec billgenie-postgres pg_isready -U billgenie -d billgenie
```

From the **API droplet**, test reachability:

```bash
nc -vz DB_PRIVATE_IP 5432
```

## 6. Point the API at the DB droplet

On the **API** droplet, edit `/opt/billgenie/.env`:

```bash
DATABASE_URL=postgresql://billgenie:YOUR_PASSWORD@DB_PRIVATE_IP:5432/billgenie?sslmode=disable
```

**Required:** `sslmode=disable` (production otherwise forces `sslmode=require`).

Redeploy API:

```bash
cd /opt/billgenie/app
git pull origin main
./scripts/deploy-droplet.sh
```

Verify:

```bash
curl -fsS https://api.thebillgenie.com/health
docker logs billgenie-api 2>&1 | grep -iE 'Database connected|migrations|Failed to connect'
```

When healthy, **destroy DigitalOcean Managed Database** in the console to stop billing.

## 7. Backups (run on the DB droplet)

```bash
/opt/billgenie/app/scripts/backup-droplet-postgres.sh
# → /opt/billgenie/backups/*.sql.gz
```

Cron:

```bash
15 3 * * * /opt/billgenie/app/scripts/backup-droplet-postgres.sh >>/var/log/billgenie-pg-backup.log 2>&1
```

Copy dumps off-box periodically.

## Sizing notes

| Stage | DB droplet |
|-------|------------|
| Pre-go-live / early | 1 GB RAM Basic |
| Growing | 2 GB+ |

API and DB must stay in the **same region + VPC**. Public IP of the DB droplet is for SSH only — apps use the **private** IP.
