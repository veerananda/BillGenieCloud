# BillGenie Platform Ops API

Creators-only endpoints for managing restaurant tenants. Used by `billgenie-platform` console.

## Configure

Add to Fly secrets or `.env`:

```bash
PLATFORM_OPS_API_KEY=<generate-long-random-string>
```

Generate (PowerShell):

```powershell
[Convert]::ToBase64String((1..48 | ForEach-Object { Get-Random -Maximum 256 }))
```

Redeploy API after setting the secret.

## Authentication

Send on every request:

```
X-Platform-Api-Key: <PLATFORM_OPS_API_KEY>
X-Platform-Actor: Veera   # optional, for audit_logs
```

Or: `Authorization: Bearer <PLATFORM_OPS_API_KEY>`

## Endpoints

### `GET /platform/restaurants`
Query: `search`, `phase`, `limit`, `offset`

### `GET /platform/restaurants/:id`
Full tenant detail, limits, usage, recent renewals.

### `POST /platform/restaurants/:id/grant-subscription`
```json
{
  "reason": "Rajahmundry pilot partner",
  "billing_cycle": "monthly",
  "duration_days": 30,
  "selection": { "operation_mode": "both", "max_tables": 10, "kitchen_dine_in": true }
}
```

### `POST /platform/restaurants/:id/extend-trial`
```json
{ "reason": "Extended evaluation", "days": 15 }
```

### `PUT /platform/restaurants/:id/selection`
Update add-ons without changing subscription end date.

### `PUT /platform/restaurants/:id/active`
```json
{ "reason": "Abuse", "is_active": false }
```

### `DELETE /platform/restaurants/:id`
Permanently deletes the restaurant and **all** related data (users, orders, menu, inventory, tables, audit logs, etc.). Irreversible.

```json
{
  "reason": "Duplicate test signup — customer requested removal",
  "confirm_name": "Exact Restaurant Name"
}
```

`confirm_name` must match the restaurant name (case-insensitive).

All mutations append to `audit_logs` with action prefix `platform_*` (except delete, which is logged server-side before cascade).
