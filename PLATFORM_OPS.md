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
  "billing_cycle": "quarterly",
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
Permanently deletes the restaurant and related data (users, orders, menu, inventory, tables, audit logs, etc.). Irreversible.

**Retained:** `trial_eligibilities` for the account email/phone so the same identity cannot claim another free trial after re-registration. They must use paid signup (`Subscribe now`).

```json
{
  "reason": "Duplicate test signup — customer requested removal",
  "confirm_name": "Exact Restaurant Name"
}
```

`confirm_name` must match the restaurant name (case-insensitive).

### `POST /platform/restaurants/:id/menu/bulk`
Upsert menu items from platform onboarding (JSON body; Excel parsed client-side).

```json
{
  "reason": "Onboarding — menu from customer spreadsheet",
  "items": [
    {
      "category": "Main Course",
      "type": "Paneer Butter Masala",
      "price": 280,
      "is_veg": true,
      "is_available": true,
      "is_readily_available": false,
      "is_taxable": true,
      "available_channels": ["dine_in", "counter_eat_here", "counter_takeaway", "swiggy", "zomato"],
      "channel_prices": {
        "dine_in": 280,
        "swiggy": 300,
        "zomato": 300
      },
      "variants": [
        {
          "label": "Half",
          "price": 160,
          "recipe_scale": 0.5,
          "is_default": false,
          "is_available": true,
          "channel_prices": { "dine_in": 160, "swiggy": 180 }
        },
        {
          "label": "Full",
          "price": 280,
          "recipe_scale": 1,
          "is_default": true
        }
      ]
    }
  ]
}
```

**Menu sheet columns:** `category`, `type`, `price`, `isVeg`, `isAvailable`, `isReadilyAvailable`, `isTaxable`, `availableChannels`, optional `price_dine_in` / `price_counter_eat_here` / `price_counter_takeaway` / `price_swiggy` / `price_zomato`.

**Portions sheet (optional):** `category`, `type`, `label`, `price`, `recipeScale`, `isDefault`, `isAvailable`, plus the same optional `price_*` channel columns. Rows match Menu by **category + type**. Omit the sheet (or leave empty) to keep / auto-create a default Full portion.

- `isTaxable`: default **true** when omitted; use `no` for MRP items.
- `availableChannels`: comma-separated channel ids; blank = all channels.
- Channel price blanks fall back to the item/portion base `price`.
- Upsert key: **category + type** (name), case-insensitive.

### `POST /platform/restaurants/:id/recipes/bulk`
Replace recipes per menu item; ingredients are auto-created for inventory (no stock columns).

```json
{
  "reason": "Onboarding — recipes from customer spreadsheet",
  "items": [
    {
      "category": "Burger",
      "type": "Veg",
      "ingredient_name": "Patty",
      "unit": "grams",
      "quantity": 120
    }
  ]
}
```

Excel columns: `category`, `type`, `Ingredient name`, `unit`, `quantity`.  
Use the same **category** and **type** values as the menu sheet (e.g. Burger + Veg vs Pizza + Veg are different items).  
Upload menu bulk **before** recipes. Each menu in the file gets its full recipe replaced.

All mutations append to `audit_logs` with action prefix `platform_*` (except delete, which is logged server-side before cascade).
