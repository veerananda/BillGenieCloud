# BillGenie Platform Scaling Roadmap

**Status:** Living document — update as tenant count and traffic grow  
**Created:** June 2026  
**Audience:** Backend, infra, and product owners  

## Purpose

This document outlines how to grow BillGenie from **one restaurant** to **~1,000 restaurants** on a shared SaaS platform without rewriting the product. It assumes the current stack:

| Component | Current production |
|-----------|-------------------|
| API + WebSocket + QR tracking | Fly.io (`billgenie-api`, region `bom`) |
| Database | DigitalOcean Managed PostgreSQL (`blr1`) |
| Redis | Upstash (`ap-south-1`) — optional today, required for multi-instance |
| Mobile app | Expo / React Native → single API URL |

The **data model is multi-tenant** (`restaurant_id` on core tables). The **deployment is single-instance pilot scale**. This roadmap closes that gap in phases.

---

## Current state (Phase 0)

**Target:** 1–10 restaurants, low concurrent devices.

| Area | Status |
|------|--------|
| Tenancy | JWT + `restaurant_id` query scoping |
| Fly | 1 machine, ~1 GB RAM, 1 shared CPU |
| WebSocket | In-memory hub; Redis fan-out **available** if `REDIS_URL` set |
| QR tracking (SSE) | In-memory `OrderTrackingHub` only — **not** Redis-backed |
| Database | Shared Postgres, GORM AutoMigrate on deploy |
| Rate limiting | Not implemented |
| Order history | Grows unbounded per tenant |

**When to leave Phase 0:** Any of the following:

- More than ~10 active restaurants
- Lunch peak feels sluggish (API latency, WS delays)
- Planning a second Fly machine
- Customer QR pages miss live updates after a deploy or scale event

---

## Phase 1 — Solid SaaS pilot (10–100 restaurants)

**Goal:** Reliable multi-tenant platform with headroom for dozens of busy lunch rushes.

### Infrastructure

| Resource | Recommendation |
|----------|----------------|
| Fly API | **2–3 machines**, `shared-cpu-1x`, **1–2 GB RAM** each |
| Fly config | `min_machines_running = 2` for HA during deploys |
| Redis | **Required** in production (`REDIS_URL` on every instance) |
| Postgres | DO **4 GB RAM** tier or equivalent; enable automated backups |
| Connection pool | Keep `MaxOpenConns(100)` per instance; 3 instances ≈ 300 DB connections — stay within Postgres limit |

### Code / platform work (priority order)

1. **Require Redis in production**  
   - WebSocket events must fan out across all Fly machines (`EventBridge` → `RedisBroker`).  
   - Checkout locks already support Redis; verify `REDIS_URL` in `fly-secrets.env`.

2. **Redis-backed QR tracking (SSE)**  
   - Today `OrderTrackingHub` is in-memory per process.  
   - With 2+ machines, kitchen updates on instance A may not reach a customer SSE connection on instance B.  
   - **Action:** Publish tracking updates to Redis channel `billgenie:track:{token}`; each instance forwards to local SSE subscribers.

3. **Database indexes** (migration)  
   Add composite indexes if not present:
   - `orders (restaurant_id, created_at DESC)`
   - `orders (restaurant_id, status, created_at DESC)`
   - `orders (restaurant_id, ticket_number, created_at)` for counter/today queries
   - `order_items (order_id, status)`
   - `tracking_token` already indexed — keep it

4. **Slim WebSocket payloads for item updates**  
   - Today kitchen status changes send **full order + all items**.  
   - **Action:** For `order_item_status_changed`, send delta: `order_id`, `item_id`, `status`, `ready_count`, `total_count` (client merges). Keep full order on `order_created` only.

5. **Rate limiting** (edge or middleware)  
   - `/auth/login`, `/auth/register`, `/public/*`, `/t/*`  
   - Suggested: 10–30 req/min per IP on auth; 60/min on public menu; tracking page by token.

6. **Production logging**  
   - Set `ENABLE_LOGGING=false` or structured JSON at `warn` in production.  
   - Reduces CPU and log volume at scale.

7. **Avoid tracking DB round-trips**  
   - `NotifyOrderTrackingUpdate` re-fetches full order; pass updated order from handler when possible.

### Frontend (light touch)

- Ensure list endpoints use **summary** APIs (`/orders/summary`, `/orders/counter/today`, `/orders/history`) — not full `listOrders` fallbacks with limit 500–1000.
- Remove or gate verbose `console.log` in production builds.

### Metrics to watch

| Metric | Warning threshold (pilot) |
|--------|---------------------------|
| API p95 latency | > 500 ms on order create |
| Fly CPU | Sustained > 70% at lunch |
| Postgres connections | > 80% of max |
| WebSocket reconnect rate | Spike after deploy |
| Redis publish errors | Any sustained errors |

### Exit criteria for Phase 1

- 2+ Fly machines with zero WS desync across devices in same restaurant
- QR tracking updates live after kitchen changes with multiple machines
- Order history p95 < 1 s for last 30 days per restaurant
- Rate limits on auth and public routes

---

## Phase 2 — Growth (100–300 restaurants)

**Goal:** Predictable performance, operational safety, controlled data growth.

### Infrastructure

| Resource | Recommendation |
|----------|----------------|
| Fly API | **3–6 machines**, autoscale on CPU or request concurrency |
| Postgres | **8–16 GB RAM**, read replica for reporting/history |
| Redis | Upstash **Pro** or dedicated Redis; monitor pub/sub throughput |
| CDN | Optional in front of static tracking HTML (`/t/{token}`) — low priority |

### Platform work

1. **Order archival**  
   - Move orders older than 90–180 days to `orders_archive` / partitioned table.  
   - Keep hot `orders` table small; history API reads archive when needed.

2. **Read replica routing**  
   - Writes → primary.  
   - `GET /orders/history`, `GET /orders/sales-summary` → replica (read-only connection).

3. **Background jobs**  
   - Nightly sales rollups per `restaurant_id`.  
   - Large exports (CSV) off the request thread (queue: Redis list, or lightweight job runner).

4. **Tenant quotas** (product + API)  
   - Max staff users, max orders/day, feature flags per plan.  
   - Prevents one tenant from dominating shared resources.

5. **Observability per tenant**  
   - Log/metric tags: `restaurant_id`, `endpoint`, `event_type`.  
   - Dashboards: top 10 tenants by API volume, WS messages, DB time.

6. **Postgres row-level security (optional but recommended)**  
   - RLS policy: `restaurant_id = current_setting('app.restaurant_id')`.  
   - Set session variable from JWT in middleware — defense in depth against query bugs.

7. **WebSocket connection limits**  
   - Per-restaurant cap (e.g. 20 connections) to prevent abuse.  
   - Graceful disconnect of stale clients.

### Mobile app

- No per-restaurant API URL change required.  
- Consider **EAS Update** for config flags (feature rollout) without full store release.

### Exit criteria for Phase 2

- 300 restaurants with no single-tenant outage affecting others
- History/report queries do not slow down order create path
- Archival job running; primary DB size growth bounded

---

## Phase 3 — Scale (300–1,000 restaurants)

**Goal:** Cost-efficient, resilient platform for ~1,000 paying restaurants.

### Infrastructure (indicative)

| Resource | Recommendation |
|----------|----------------|
| Fly API | **6–12+ machines**, multi-region only if expanding outside India |
| Postgres | **16–32 GB+** primary; 1–2 read replicas; consider **partitioning** `orders` by month |
| Redis | Dedicated cluster; separate channels for WS vs tracking vs jobs |
| Object storage | Receipt PDFs, exports, QR campaign assets (S3 / DO Spaces) |

### Platform work

1. **Event payload budget**  
   - Target < 5 KB per WS message for hot paths.  
   - Full order snapshots only on create/sync, not every item tap.

2. **Tracking at scale**  
   - SSE keepalive every 25 s × thousands of customers = many idle connections.  
   - Consider: shorter TTL (2 h), or switch customers to **polling** `/t/{token}/status` every 10–15 s if connection count hurts.

3. **API gateway / WAF**  
   - Cloudflare or Fly edge: DDoS, bot protection, global rate limits.

4. **Database connection pooling**  
   - **PgBouncer** (transaction mode) between Fly fleet and Postgres.  
   - Allows hundreds of app connections without hundreds of Postgres backends.

5. **Zero-downtime migrations**  
   - Stop relying on AutoMigrate at process start for large tables.  
   - Versioned migrations (goose / golang-migrate) run in CI/CD before deploy.

6. **Disaster recovery**  
   - RPO/RTO documented: backup restore drill quarterly.  
   - Redis treated as ephemeral (WS/tracking can rebuild); Postgres is source of truth.

7. **Commercial ops**  
   - Billing (Stripe/Razorpay subscriptions) tied to `restaurant_id`.  
   - Suspend API access for unpaid tenants without affecting others.

### Capacity planning (rough)

Assume **lunch peak**:

| Variable | Conservative estimate |
|----------|---------------------|
| Active restaurants at peak | 30% of 1,000 = **300** |
| Devices per restaurant | **3** (counter, kitchen, manager) |
| WebSocket connections | **~900** staff + customer SSE (variable) |
| Order creates per minute (platform) | **50–150** |
| Item status updates per minute | **200–600** |

One **2 GB / 2 CPU** Fly machine handles ~100–200 concurrent WS with headroom for HTTP. Plan **6–10 machines** at 1,000 restaurants with autoscale rules.

**Postgres:** 1,000 restaurants × 50 orders/day × 365 ≈ **18M orders/year** — archival/partitioning is mandatory.

---

## Architecture diagram (target end state)

```
                    ┌─────────────────┐
                    │  Mobile app     │
                    │  (all tenants)  │
                    └────────┬────────┘
                             │ HTTPS / WSS
                    ┌────────▼────────┐
                    │ Fly.io (N VMs)  │
                    │  billgenie-api  │
                    └────────┬────────┘
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼────────┐    │    ┌─────────▼─────────┐
     │ Upstash Redis   │◄───┘    │ DO Postgres       │
     │ WS + track pub  │         │ primary + replica │
     └─────────────────┘         └───────────────────┘
```

---

## What not to do early

| Anti-pattern | Why |
|--------------|-----|
| One Fly machine per restaurant | Ops nightmare, cost explosion |
| Separate database per tenant at 100+ | Migration and reporting pain |
| Shard by restaurant before 500+ tenants | Premature complexity |
| Full order broadcast on every kitchen tap | Bandwidth and battery cost at scale |
| Skip Redis and scale horizontally | WS and SSE desync |

---

## Checklist summary

### Before 10 restaurants
- [ ] `REDIS_URL` set in production
- [ ] 2 Fly machines minimum
- [ ] Composite DB indexes on `orders`
- [ ] Rate limits on auth + public routes

### Before 100 restaurants
- [ ] Redis-backed SSE tracking hub
- [ ] Slim WS payloads for item status
- [ ] Production logging tuned down
- [ ] Monitoring (latency, CPU, DB connections, Redis errors)

### Before 300 restaurants
- [ ] Order archival strategy live
- [ ] Read replica for history/sales
- [ ] Per-tenant metrics and quotas
- [ ] Background jobs for heavy reports

### Before 1,000 restaurants
- [ ] PgBouncer or equivalent
- [ ] Partitioned or archived order storage
- [ ] 6+ API instances with autoscale
- [ ] WAF / edge rate limiting
- [ ] Billing + tenant suspension
- [ ] Migration pipeline (not startup AutoMigrate)

---

## Related docs

- [DEPLOY_FLY.md](./DEPLOY_FLY.md) — current Fly deploy
- [DEPLOYMENT_GUIDE.md](./DEPLOYMENT_GUIDE.md) — production stack overview
- [BACKEND_STATUS.md](./BACKEND_STATUS.md) — feature checklist

---

## Revision history

| Date | Change |
|------|--------|
| 2026-06 | Initial roadmap: Phase 0 → 1,000 restaurants |
