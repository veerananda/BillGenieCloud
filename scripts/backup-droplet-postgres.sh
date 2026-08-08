#!/usr/bin/env bash
# Daily (or on-demand) pg_dump of droplet-local BillGenie Postgres.
#
# Usage (on droplet):
#   ./scripts/backup-droplet-postgres.sh
#
# Cron example (daily 03:15 UTC):
#   15 3 * * * /opt/billgenie/app/scripts/backup-droplet-postgres.sh >>/var/log/billgenie-pg-backup.log 2>&1
#
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/opt/billgenie/backups}"
KEEP_DAYS="${KEEP_DAYS:-14}"
DB_ENV="${DB_ENV:-/opt/billgenie/db.env}"
CONTAINER="${POSTGRES_CONTAINER:-billgenie-postgres}"

log() { echo "[pg-backup] $*"; }
die() { echo "[pg-backup] ERROR: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

need_cmd docker
[[ -f "$DB_ENV" ]] || die "db env not found: $DB_ENV"
docker inspect "$CONTAINER" >/dev/null 2>&1 || die "container not running: $CONTAINER"

# shellcheck disable=SC1090
set -a
# shellcheck source=/dev/null
source "$DB_ENV"
set +a

USER_NAME="${POSTGRES_USER:-billgenie}"
DB_NAME="${POSTGRES_DB:-billgenie}"
[[ -n "${POSTGRES_PASSWORD:-}" ]] || die "POSTGRES_PASSWORD empty in $DB_ENV"

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="${BACKUP_DIR}/billgenie-${STAMP}.sql.gz"

log "dumping ${DB_NAME} from ${CONTAINER} → ${OUT}"
docker exec -e PGPASSWORD="$POSTGRES_PASSWORD" "$CONTAINER" \
  pg_dump -U "$USER_NAME" -d "$DB_NAME" --no-owner --no-acl \
  | gzip -c >"$OUT"

chmod 600 "$OUT"
log "wrote $(du -h "$OUT" | awk '{print $1}')"

# Prune old dumps
find "$BACKUP_DIR" -type f -name 'billgenie-*.sql.gz' -mtime "+${KEEP_DAYS}" -print -delete \
  | while read -r f; do log "pruned $f"; done || true

log "done (retain ${KEEP_DAYS} days)"
