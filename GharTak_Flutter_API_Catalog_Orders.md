# GharTak — Flutter API integration: Zones, catalog & orders

**Index:** `GharTak_Flutter_Integration_Index.md`  
**Maps / geo:** `GharTak_Flutter_Mapbox_Guide.md`  
**Demo IDs:** `GharTak_Flutter_Demo_Seed_Guide.md`  
**Contract:** `cmd/api/openapi.yaml`

Integrate after auth. Cart stays on the device until `POST /orders`.

---

## 1. Flutter placement

```
features/zones/
features/merchants/
features/orders/
features/addresses/   # if you use saved addresses APIs
```

Each feature: `data` (DTO + API + repository) → `domain` → existing UI. Share money parsing (`String` decimals) in `core`.

---

## 2. Public catalog path (no auth)

### Zones

```http
GET /zones
GET /zones?active=true
```

Phase 1 demo: Attock + Hasan Abdal. Fields include `id`, `city_name`, `slug`, `center_lat`, `center_lng`, `service_radius_km`, fee card fields, `is_active`.

### Merchants in a zone

```http
GET /merchants?zone_id=11111111-1111-4111-8111-111111111101
```

Returns **approved** merchants only (max 20). Demo kitchen ID:

`22222222-2222-4222-8222-222222222201`

### Menu

```http
GET /merchants/{id}/catalog
```

Use returned item `id` values in the cart. Seeded names: Chicken Biryani `450.00`, Seekh Kabab `250.00` — **UUIDs are not fixed**.

---

## 3. Customer order path (Bearer customer)

### Quote — fee preview (no payment, no order row)

```http
POST /orders/quote
Authorization: Bearer <access_token>
```

```json
{
  "type": "food",
  "zone_id": "11111111-1111-4111-8111-111111111101",
  "merchant_id": "22222222-2222-4222-8222-222222222201",
  "drop_lat": 33.780,
  "drop_lng": 72.370,
  "drop_address": "House 12, Attock City",
  "items": [
    { "catalog_item_id": "<from catalog>", "quantity": 2 }
  ],
  "payment_method": "cod"
}
```

Rules:

- Do **not** send `distance_km`.
- `type`: `food` | `mart` | `pharmacy` | `courier` | `errand` (match OpenAPI enums).
- Food/mart/pharmacy require `merchant_id` + items.
- Courier/errand: no merchant; use pickup + drop + effort as documented in OpenAPI.
- Quote works even if `phone_verified` is false.

Response (use for checkout UI + map line):

```json
{
  "data": {
    "distance_km": "3.42",
    "duration_min": 12,
    "item_total": "900.00",
    "delivery_fee": "…",
    "commission_amount": "…",
    "rider_earning": "…",
    "surge_multiplier": "1.00",
    "total": "…",
    "approximate": false,
    "route": { "type": "LineString", "coordinates": [[72.36, 33.77], [72.37, 33.78]] }
  }
}
```

Show an “estimated” chip when `approximate` is true.

### Place — create order

Same body as quote **plus**:

```json
{ "client_request_id": "req-food-0001" }
```

- `client_request_id`: 8–64 chars, unique per attempt intent. Retries with the **same** id return the original order (idempotent).
- Prefer `cod` for demos (no gateway).
- If `phone_verified` is false → **409** `{ "error": { "code": "phone_required", ... } }` — run phone link (auth guide), then retry with the **same** `client_request_id`.

### Get order

```http
GET /orders/{id}
Authorization: Bearer <access_token>
```

### Cancel

```http
POST /orders/{id}/cancel
```

(See OpenAPI for body / allowed statuses.)

---

## 4. Recommended Flutter checkout flow

```
Select zone → load merchants → load catalog → build local cart
     → drop pin (validate inside zone radius)
     → optional geo reverse for address hint → user edits text
     → POST /orders/quote → show totals + route
     → confirm → POST /orders with new client_request_id
     → navigate to order detail / tracking stub
```

Generate `client_request_id` once per confirm tap (UUID without dashes or `req-<uuid>`). Persist it until success or explicit “new attempt”.

---

## 5. Money & JSON conventions

| Topic | Rule |
|---|---|
| Money fields | `String` like `"450.00"` — never `double` |
| Success | `{ "data": ... }` |
| Error | `{ "error": { "code", "message" } }` |
| Keys | `snake_case` in JSON; map to your Dart style in DTOs |

---

## 6. Related endpoints (second pass)

| Area | Paths | Demo priority |
|---|---|---|
| Saved addresses | `GET/POST /addresses`, `DELETE /addresses/{id}` | Medium |
| Merchant order board | `GET /merchants/orders`, actions under `/merchants/orders/{id}/{action}` | Medium (merchant app) |
| Ratings | `POST /orders/{id}/ratings` | Low |
| Chat | `GET/POST /orders/{id}/messages`, `WS /ws/chat/{order_id}` | Low |
| Live location | `WS /ws/location/{order_id}` | Needs rider |
| Uploads | `POST /uploads/presign` then PUT JPEG | Onboarding / POD |

---

## 7. Acceptance checks

- [ ] Active zones render; Attock camera uses `center_*`.
- [ ] Demo merchant appears for Attock zone id.
- [ ] Catalog loads; cart uses live item ids.
- [ ] Quote totals match UI strings from `data` (no client fee math).
- [ ] Place with `cod` returns an order; repeat same `client_request_id` returns same id.
- [ ] Pin outside 8 km fails gracefully (UI + API).
- [ ] Repository unit tests cover DTO parsing for quote `approximate` + missing `route`.
