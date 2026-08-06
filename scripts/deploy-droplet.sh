#!/usr/bin/env bash
# Deploy BillGenie API on a DigitalOcean Droplet (Docker + nginx upstream).
#
# Features:
#   - Images tagged as billgenie-api:<git-sha> and billgenie-api:latest
#   - Blue/green on 127.0.0.1:3000 ↔ :3001 with health check before cutover
#   - nginx upstream swap for near-zero downtime
#   - Rollback: ./scripts/deploy-droplet.sh --image billgenie-api:<sha>
#
# Usage (on droplet, from repo root or any cwd):
#   ./scripts/deploy-droplet.sh
#   ./scripts/deploy-droplet.sh abc1234
#   ./scripts/deploy-droplet.sh --image billgenie-api:abc1234
#
set -euo pipefail

APP_NAME="${APP_NAME:-billgenie-api}"
ENV_FILE="${ENV_FILE:-/opt/billgenie/.env}"
UPSTREAM_CONF="${UPSTREAM_CONF:-/etc/nginx/conf.d/billgenie-upstream.conf}"
HEALTH_PATH="${HEALTH_PATH:-/health}"
HEALTH_RETRIES="${HEALTH_RETRIES:-30}"
HEALTH_SLEEP_SEC="${HEALTH_SLEEP_SEC:-2}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

log() { echo "[deploy] $*"; }
die() { echo "[deploy] ERROR: $*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

need_cmd docker
need_cmd curl
need_cmd nginx

[[ -f "$ENV_FILE" ]] || die "env file not found: $ENV_FILE"
[[ -f "$APP_DIR/Dockerfile" ]] || die "Dockerfile not found in $APP_DIR"

if ! grep -R -q 'billgenie_backend' /etc/nginx/sites-enabled/ 2>/dev/null; then
  die "nginx site must proxy_pass http://billgenie_backend (see DEPLOY_DROPLET.md one-time setup)"
fi

IMAGE_REF=""
SHA_ARG=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image)
      IMAGE_REF="${2:-}"
      shift 2
      ;;
    -h|--help)
      sed -n '2,20p' "$0"
      exit 0
      ;;
    *)
      SHA_ARG="$1"
      shift
      ;;
  esac
done

cd "$APP_DIR"

if [[ -z "$IMAGE_REF" ]]; then
  if [[ -n "$SHA_ARG" ]]; then
    GIT_SHA="$SHA_ARG"
  else
    GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || true)"
    [[ -n "$GIT_SHA" ]] || die "not a git repo and no sha/--image given"
  fi
  IMAGE_REF="${APP_NAME}:${GIT_SHA}"
  BUILD=1
else
  GIT_SHA="${IMAGE_REF##*:}"
  BUILD=0
fi

NEXT_NAME="${APP_NAME}-next"
OLD_NAME="$APP_NAME"

read_active_port() {
  if [[ -f "$UPSTREAM_CONF" ]]; then
    local port
    port="$(grep -Eo '127\.0\.0\.1:[0-9]+' "$UPSTREAM_CONF" | head -1 | cut -d: -f2 || true)"
    if [[ "$port" == "3000" || "$port" == "3001" ]]; then
      echo "$port"
      return
    fi
  fi
  # Infer from running container publish
  local mapped
  mapped="$(docker port "$OLD_NAME" 3000 2>/dev/null | head -1 || true)"
  if [[ "$mapped" =~ :3001$ ]]; then
    echo "3001"
  else
    echo "3000"
  fi
}

write_upstream() {
  local port="$1"
  cat >"$UPSTREAM_CONF" <<EOF
# Managed by scripts/deploy-droplet.sh — do not edit by hand while deploys run
upstream billgenie_backend {
    server 127.0.0.1:${port};
    keepalive 32;
}
EOF
  nginx -t
  systemctl reload nginx
  log "nginx upstream → 127.0.0.1:${port}"
}

wait_healthy() {
  local port="$1"
  local i
  for i in $(seq 1 "$HEALTH_RETRIES"); do
    if curl -fsS "http://127.0.0.1:${port}${HEALTH_PATH}" >/dev/null 2>&1; then
      log "health OK on :${port} (attempt ${i})"
      return 0
    fi
    sleep "$HEALTH_SLEEP_SEC"
  done
  die "health check failed on :${port} after ${HEALTH_RETRIES} attempts"
}

ACTIVE_PORT="$(read_active_port)"
if [[ "$ACTIVE_PORT" == "3000" ]]; then
  NEXT_PORT="3001"
else
  NEXT_PORT="3000"
fi

log "app dir: $APP_DIR"
log "image:   $IMAGE_REF"
log "active:  :${ACTIVE_PORT}  →  next: :${NEXT_PORT}"

if [[ "$BUILD" -eq 1 ]]; then
  log "building $IMAGE_REF (+ ${APP_NAME}:latest)"
  docker build \
    -t "$IMAGE_REF" \
    -t "${APP_NAME}:latest" \
    "$APP_DIR"
else
  docker image inspect "$IMAGE_REF" >/dev/null 2>&1 \
    || die "image not found locally: $IMAGE_REF (build first or pull)"
  docker tag "$IMAGE_REF" "${APP_NAME}:latest"
fi

log "removing any leftover ${NEXT_NAME}"
docker rm -f "$NEXT_NAME" >/dev/null 2>&1 || true

log "starting ${NEXT_NAME} on 127.0.0.1:${NEXT_PORT}"
docker run -d \
  --name "$NEXT_NAME" \
  --restart unless-stopped \
  --env-file "$ENV_FILE" \
  -e ENABLE_LOGGING="${ENABLE_LOGGING:-true}" \
  -p "127.0.0.1:${NEXT_PORT}:3000" \
  "$IMAGE_REF" >/dev/null

wait_healthy "$NEXT_PORT"

log "cutting over nginx"
write_upstream "$NEXT_PORT"

log "retiring old container (if any)"
docker rm -f "$OLD_NAME" >/dev/null 2>&1 || true
docker rename "$NEXT_NAME" "$OLD_NAME"

log "public health check"
if curl -fsS "https://api.thebillgenie.com${HEALTH_PATH}" >/dev/null 2>&1 \
  || curl -fsS "http://127.0.0.1:${NEXT_PORT}${HEALTH_PATH}" >/dev/null 2>&1; then
  log "deploy OK  image=$IMAGE_REF  port=${NEXT_PORT}"
else
  log "WARN: public URL health failed; container on :${NEXT_PORT} is healthy"
fi

docker ps --filter "name=${APP_NAME}" --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
log "rollback tip: $0 --image ${APP_NAME}:<previous-sha>"
