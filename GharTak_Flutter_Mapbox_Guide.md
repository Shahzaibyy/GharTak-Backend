# GharTak — Flutter Mapbox guide (client SDK + backend)

**Companion to:** `GharTak_Backend_Mapbox_Requirements.md` (implemented on this API).  
**Index:** `GharTak_Flutter_Integration_Index.md`.

Flutter owns **map rendering**. This backend owns **road distance, ETA, route geometry for pricing, and Pakistan-biased geocoding**. Putting a secret Directions token in the app would break pricing integrity and burn Mapbox quota.

---

## 1. Two tokens (do not mix them)

| Token | Where it lives | Purpose |
|---|---|---|
| **Maps SDK public token** | Flutter (`--dart-define` / flavor) | Map tiles, camera, markers, local gestures |
| **`MAPBOX_BACKEND_TOKEN`** | Render / `.env` on API only | Directions + Geocoding v6 (`country=pk`) |

If the backend token is empty:

- `POST /orders/quote` and `POST /orders` still work using haversine × `ROUTE_CIRCUITY_FACTOR` with `"approximate": true`.
- `GET /geo/search` and `GET /geo/reverse` return **unavailable**.
- FE should still place orders; show “estimated distance” when `approximate` is true.

---

## 2. Ownership split

| Concern | Flutter Mapbox SDK | Backend API |
|---|---|---|
| Show map / style / gestures | Yes | No |
| Camera lock to zone | Use `center_lat` / `center_lng` / `service_radius_km` from `GET /zones` | Serves zone geometry |
| Merchant markers | Plot DB merchants (`GET /merchants`) | Never Mapbox POIs |
| Address search typeahead | Call `GET /geo/search` | Proxies Geocoding v6 |
| Reverse after pan/idle | Debounced `GET /geo/reverse` | Proxies reverse |
| Delivery fee / ETA / polyline for checkout | Draw `Quote.route` if present | `POST /orders/quote` |
| Live rider marker | Consume WS frames | `GET /ws/location/{order_id}` |
| Turn-by-turn navigation | Hand off to external maps if product requires | Out of scope for MVP pricing |

**Never send** client-computed `distance_km`, fees, or surge in quote/place bodies.

---

## 3. Backend endpoints for maps

Base URL example: `https://your-api.onrender.com` (or `http://10.0.2.2:8080` for Android emulator → host).

### 3.1 Zones (public) — camera + service area

```http
GET /zones?active=true
Authorization: not required
```

Use for each zone:

- `center_lat`, `center_lng` → initial camera
- `service_radius_km` → clamp pin / show out-of-area UI (Attock seed radius is `8.00`)
- `id` → pass as `zone_id` on quote/place and optional `zone_id` on geo search

Hardcoded Attock for demos: `11111111-1111-4111-8111-111111111101` @ `33.7667, 72.3667`.

### 3.2 Forward geocode (Bearer)

```http
GET /geo/search?q=attock&near_lat=33.77&near_lng=72.36&limit=5&zone_id=<uuid>
Authorization: Bearer <access_token>
```

- `q`: 3–100 chars.
- Prefer `near_*` or `zone_id` so results bias to the district.
- Response `data[]`: `{ label, lat, lng, place }`.
- Treat as **hint only**. After the user picks a row, put the pin on that lat/lng and let them edit the address string.

Rate limit (server): ~30 / minute / account. Debounce UI ≥ 300 ms; cancel in-flight requests on new keystrokes.

### 3.3 Reverse geocode (Bearer)

```http
GET /geo/reverse?lat=33.78&lng=72.37
Authorization: Bearer <access_token>
```

Call on **map idle** (not every frame). Prefill address field; keep the pin as source of truth.

Rate limit: ~20 / minute / account.

### 3.4 Quote (Bearer customer) — fee + route line

```http
POST /orders/quote
Authorization: Bearer <access_token>
Content-Type: application/json
```

Body (food example — catalog IDs from API, not hardcoded):

```json
{
  "type": "food",
  "zone_id": "11111111-1111-4111-8111-111111111101",
  "merchant_id": "22222222-2222-4222-8222-222222222201",
  "drop_lat": 33.780,
  "drop_lng": 72.370,
  "drop_address": "House 12, Near Clock Tower, Attock",
  "items": [{ "catalog_item_id": "<uuid-from-catalog>", "quantity": 1 }],
  "payment_method": "cod"
}
```

Response fields Flutter should use:

| Field | UI use |
|---|---|
| `distance_km` | Display only (string) |
| `duration_min` | “~N min” |
| `item_total`, `delivery_fee`, `total` | Checkout breakdown (strings) |
| `approximate` | Badge “estimated” when true |
| `route` | Optional GeoJSON LineString → `GeoJsonSource` / line layer |

Coordinates in GeoJSON are **`[lng, lat]`**.

### 3.5 Live location (WebSocket)

```
GET /ws/location/{order_id}
Authorization: Bearer <access_token>
```

Rider publishes `{ "lat", "lng" }` (+ optional heading/speed). Customer/merchant receive fan-out frames; may include `eta_min` when the backend refreshes ETA. Reconnect with backoff; do not open a socket until the order has an assigned rider if your UX only shows tracking after accept.

---

## 4. Flutter implementation checklist

1. **Feature module `map`:** Mapbox widget, camera helpers, zone circle, pin drag.
2. **Feature module `geo`:** repository wrapping `/geo/search` and `/geo/reverse` only (no Mapbox REST from the phone).
3. **Feature module `orders`:** quote → parse `route` → pass geometry to `map`.
4. **Out-of-area:** if haversine from zone center > `service_radius_km`, block checkout with a clear message (backend also rejects out-of-area).
5. **Persist address:** `{ lat, lng, address_text }` in local cart / address book — not raw geocoder JSON.
6. **Errors:** map `unavailable` on geo to “Address search temporarily offline — drop a pin instead.”
7. **Tests:** fake repositories; never hit real Mapbox or the API in widget tests.

Suggested packages (adapt to your rules): Mapbox Maps Flutter SDK for display; your existing HTTP client for REST; `web_socket_channel` (or equivalent) for location WS.

---

## 5. Backend env the FE team must ask for

Share with whoever runs Render / local API:

```env
MAPBOX_BACKEND_TOKEN=<secret server token>
MAPBOX_ROUTE_CACHE_TTL=6h
MAPBOX_GEO_SEARCH_TTL=24h
MAPBOX_GEO_REVERSE_TTL=168h
ROUTE_CIRCUITY_FACTOR=1.3
ROUTE_FALLBACK_KMH=20
```

Flutter only needs:

```text
MAPBOX_ACCESS_TOKEN=<public maps token>
API_BASE_URL=https://...
```

---

## 6. Demo without a backend Mapbox token

Still showcase-ready:

1. Zones + camera lock work (DB).
2. User drops pin; skip search or show offline copy.
3. Quote returns fees with `approximate: true`.
4. Skip drawing a road polyline or draw a straight dashed line client-side for presentation only (label it estimated).

Do **not** invent fees on the client for the demo — always call `/orders/quote`.
