# GharTak — Backend Requirements for Mapbox (Go / chi / Render)

**Companion to:** `GharTak_Flutter_Mapbox_Guide.md` (Flutter Maps SDK + which calls hit this API).  
**Flutter set:** start at `GharTak_Flutter_Integration_Index.md`.
**Follows:** `GharTak_Backend_Rules.md` (chi, handler → service → repository, guard clauses, ≤ 4 branches per function, context timeouts, Redis TTL on every key, money as paisa, no `float64` for money).
**Verified against:** Mapbox API docs (rate limits, Geocoding v6 params). Endpoint shapes below are **proposals** fitted to your `GharTak_Next_Steps.md` conventions (`{"data":...}`, `snake_case`, errors via `httpx.WriteError`). Not compiled.

---

## 0. TL;DR — what the backend must do

| # | Need | Why |
|---|---|---|
| 1 | `internal/platform/mapbox` client (Directions + Geocoding) behind interfaces | Flutter SDK renders maps only; it can't route or geocode |
| 2 | **Road distance + ETA + route geometry** from Directions, cached in Redis | Pricing (Step 5) needs `distance_km`; app needs route line + ETA |
| 3 | `GET /orders/quote` returns fee + ETA + route GeoJSON | One call for the checkout screen, price computed server-side |
| 4 | `GET /geo/search` and `GET /geo/reverse` proxies | Keeps token off the phone, adds `country=pk`, district bias, caching, rate limits |
| 5 | Live location payload contract + validation (Step 9) | Marker + ETA on customer map |
| 6 | Zone geometry exposed on `GET /zones` | Flutter centers/locks the camera and validates service area |
| 7 | Fallback when Mapbox fails (haversine) | Orders must not break because of a 3rd party |
| 8 | Config, secrets, usage guard, Render notes | Cost + safety |

---

## 1. Mapbox facts that shape the design

| Fact | Impact |
|---|---|
| Rate limits are counted **per access token**: Directions 300 req/min, 25 coordinates/request; Matrix 60 req/min (driving), 25 coords; Matrix `driving-traffic` 30 req/min, 10 coords; HTTP **429** on breach | Use a dedicated backend token, cache aggressively, handle 429 with backoff + fallback |
| Rate-limit headers: `X-Rate-Limit-Interval`, `X-Rate-Limit-Limit`, `X-Rate-Limit-Reset` | Log them (no tokens) to see how close you are |
| Geocoding v6: forward `…/search/geocode/v6/forward`, reverse `…/search/geocode/v6/reverse`; params `country` (ISO alpha-2), `proximity` (`lon,lat`), `limit`, `permanent` | Always send `country=pk` and `proximity` |
| Geocoding **temporary by default**; `permanent=true` is required if you intend to **store** results (needs billing set up) | See §4.3 — matters because orders/addresses persist address text |
| Geocoding v6 has **no POI data** (Search Box API is for POI) | Merchants come from **our DB**, not Mapbox POIs |
| Maps SDK on mobile is billed by MAU, separate from the above | Backend cost = Directions + Geocoding (+ Matrix if used) requests only |

**Coverage caveat:** Mapbox data for smaller Attock towns/villages may have thin address coverage. Treat geocoding as a *hint*; the map pin + user-edited address text is the source of truth (§4.3). Test real addresses in Hazro, Jand, Pindi Gheb before launch.

---

## 2. Package layout

```
internal/platform/mapbox/
  client.go        # http.Client wrapper: base URL, token, timeout, 429 handling
  directions.go    # Router implementation
  geocoding.go     # Geocoder implementation
  types.go         # Point, Route, GeoResult
  errors.go        # ErrUpstreamUnavailable, ErrRateLimited, ErrNoRoute, ErrBadInput
internal/platform/geo/
  haversine.go     # pure fallback distance + bbox/zone containment helpers
internal/modules/orders/   # uses mapbox.Router via interface (pricing + quote)
internal/modules/geo/      # NEW thin module: /geo/search, /geo/reverse handlers+service
```

Why a new `geo` module: search/reverse are customer-facing endpoints with their own rate limits and caching; they don't belong in `orders`. `platform/mapbox` stays a pure client (no business rules), consistent with `platform/pii`, `platform/money`, `platform/uploads`.

### 2.1 Interfaces (small, per Interface Segregation)

```go
// internal/platform/mapbox/types.go
type Point struct{ Lat, Lng float64 } // coordinates are not money; float64 is fine here

type Route struct {
    DistanceM  int            // meters
    DurationS  int            // seconds
    Geometry   json.RawMessage // GeoJSON LineString, passed straight to the client
}

type Router interface {
    Route(ctx context.Context, from, to Point) (Route, error)
}

type GeoResult struct {
    Label    string
    Point    Point
    Place    string // locality/district context
}

type Geocoder interface {
    Search(ctx context.Context, q string, near Point, limit int) ([]GeoResult, error)
    Reverse(ctx context.Context, p Point) (GeoResult, error)
}
```

Consumers depend on these interfaces; tests use fakes. Wire concrete clients in `cmd/api/main.go` like Redis/pgx (no globals, no `init()`).

---

## 3. Directions: distance, ETA, route line

### 3.1 Upstream request

```
GET https://api.mapbox.com/directions/v5/mapbox/driving/{lng},{lat};{lng},{lat}
    ?geometries=geojson&overview=full&alternatives=false&steps=false&access_token=<BACKEND_TOKEN>
```

- Coordinates are **`lng,lat`** order.
- `geometries=geojson` → the response `routes[0].geometry` is already a GeoJSON LineString. Return it untouched; Flutter feeds it to a `GeoJsonSource` with no decoding.
- Profile: `driving` for MVP (riders are on bikes; Mapbox has no motorbike profile — road distance is close enough). `driving-traffic` is more restricted (lower Matrix limits) and traffic data in rural Attock is unlikely to add value — start with `driving`.
- Use `distance` (meters) and `duration` (seconds) from `routes[0]`.
- `steps=false` — you don't need maneuvers (turn-by-turn is handed to Google Maps in the app).

### 3.2 Caching (Redis, TTL mandatory)

```
key:  route:{round(fromLat,4)}:{round(fromLng,4)}:{round(toLat,4)}:{round(toLng,4)}
val:  JSON {distance_m, duration_s, geometry}
TTL:  6h   (roads don't change; ETA without live traffic is stable)
```

- 4-decimal rounding ≈ 11 m grid → pickup/drop pairs for the same merchant and nearby drops share entries.
- Plain `SET key val EX 21600` is correct here (not a hash): the value is read whole.
- Redis flushed ⇒ only a cache miss, no data loss (rule §6: Redis is never the source of truth).
- Coalesce concurrent identical misses with `golang.org/x/sync/singleflight` so a burst of quotes for one pair makes one upstream call.

### 3.3 Failure / fallback (do not let Mapbox take down ordering)

```go
// orders service helper — keep ≤ 4 branches
func (s *Service) road(ctx context.Context, from, to mapbox.Point) (mapbox.Route, bool, error)
```

Behavior:
1. cache hit → return.
2. call `Router.Route` with a **short timeout** (e.g. 3 s, via `context.WithTimeout`).
3. on `ErrRateLimited` / `ErrUpstreamUnavailable` / timeout → fallback: `distance = haversine × 1.3` (circuity factor — **tune** after comparing to real routes in Attock), `duration = distance / assumed 20 km/h`, `Geometry = nil`, and mark `approximate = true` in the quote response so the app can skip drawing a line.
4. `ErrNoRoute` / bad input → domain error (422), not fallback.

The haversine fallback lives in `platform/geo`, is pure, and is table-tested.

### 3.4 Circuit-breaker light

If N consecutive upstream failures in a minute, skip the call for 30 s (in-memory counter or Redis key with TTL). Prevents each request from burning its timeout during an outage.

---

## 4. Endpoints

All follow your conventions: success `{"data": …}`, errors `{"error":{"code","message"}}`, validation **once in the handler**, JWT middleware for any authenticated role.

### 4.1 `GET /orders/quote`  (Step 5 "quote endpoint")

Query: `type`, `pickup_lat`, `pickup_lng`, `drop_lat`, `drop_lng`, `zone_id`, optional `effort` (errand/courier: `1.00|1.30|1.60`), optional `merchant_id` (food/mart → pickup comes from the merchant row, **ignore client pickup**).

Response:

```json
{
  "data": {
    "distance_km": "3.42",
    "duration_min": 11,
    "delivery_fee": "148.00",
    "surge_multiplier": "1.00",
    "approximate": false,
    "route": { "type": "LineString", "coordinates": [[72.9,33.7],[72.91,33.71]] }
  }
}
```

Rules:
- **Server computes distance. The client never sends `distance_km`.** Price = zone base + per-km × distance → effort → surge (existing locked decision), using `money` paisa helpers; JSON fees stay decimal strings.
- `distance_km` rounding: decide once (e.g. 2 decimals, half-up) and use the **same** value for the quote and the persisted order, so the customer sees what they pay.
- Validate: lat in [-90,90], lng in [-180,180]; **pickup and drop inside the zone's service radius** (see §6) → else `422 out_of_service_area`.
- Rate-limit per account (each call can cost a Directions request on cache miss).
- Returning `route` here lets the checkout screen draw the preview with **zero** extra calls.

### 4.2 `POST /orders` — consistency with the quote

`POST /orders` must **recompute** distance/fee server-side (never trust a fee or distance in the body). With the cache this is usually a hit from the quote seconds earlier. Persist `distance_km` (column already in Step 5) in the same transaction as the order insert.

### 4.3 `GET /geo/search` and `GET /geo/reverse`

```
GET /geo/search?q=<text>&near_lat=&near_lng=&limit=5   → [{label, lat, lng, place}]
GET /geo/reverse?lat=&lng=                              → {label, lat, lng, place}
```

Service adds: `country=pk`, `proximity={near_lng},{near_lat}` (fall back to the zone center), `limit` capped (≤ 5), language as needed.

Handler validation (once): `q` length 3–100 after trim; lat/lng ranges; `limit` bounds.

Caching:
- search: `geo:s:{sha1(lower(q))}:{round(near,2)}` TTL 24h
- reverse: `geo:r:{round(lat,4)}:{round(lng,4)}` TTL 7d

Cost control: per-account limit, e.g. 30 searches/min and 20 reverses/min (tune). The app only calls reverse on map **idle**, debounced (Flutter guide §8).

**Storing addresses — pick one approach deliberately:**

| Approach | Notes |
|---|---|
| **A (recommended):** store `lat`/`lng` from the **map pin** + the **text the user typed/edited**, call geocoding **temporary** | Pin coordinates are not geocoder output; user-typed text is the user's data. Matches your "orders copy lat/lng/text" decision (Step 3). Cheapest. |
| B: store geocoder labels/coordinates | Then use `permanent=true` on those requests (Mapbox requires it for storage; needs billing on the account and costs more). Check Mapbox's Terms/pricing before choosing. |

Go with A unless you have a reason to persist geocoder output. Reconfirm the exact wording of the storage terms on Mapbox's Geocoding page before launch.

### 4.4 `GET /zones` — extend response

Flutter needs this to center and lock the camera. If `zones` has no center columns, add migration `000N_zone_geo.up.sql`:

```sql
ALTER TABLE zones
  ADD COLUMN center_lat DOUBLE PRECISION,
  ADD COLUMN center_lng DOUBLE PRECISION;
-- backfill the 6 seeded cities with real centers, then add NOT NULL
```

(`service_radius_km` already exists — seed shows radius `8.00`.) Return: `{id, city_name, center_lat, center_lng, service_radius_km, is_active}`. The contract test (`internal/platform/schema/contract_test.go`) must stay green — update Go constants/lists alongside the migration. Matching `.down.sql` required; don't touch `0001`–`0006`.

Optional later: a `bbox` or polygon column for exact boundaries; radius check is enough for MVP.

---

## 5. Live location (Step 9) — contract for the map

Socket: `WS /ws/location/{order_id}` on the **root mux, outside the 15 s HTTP timeout** (already a locked decision).

Rider → server (every 5–10 s):

```json
{ "lat": 33.7712, "lng": 72.7551, "heading": 87.5, "speed": 6.2, "ts": 1790000000000 }
```

Server → customer (via Redis pub/sub `order:{id}:location`):

```json
{ "lat": 33.7712, "lng": 72.7551, "heading": 87.5, "ts": 1790000000000, "eta_min": 7 }
```

Backend rules:
1. **Authorize** the socket: only the assigned rider can publish; only the order's customer (and admin) can subscribe. Reject by `order.rider_id` / `customer_id`, not by URL knowledge.
2. **Validate** each frame once at the socket boundary: lat/lng ranges, `ts` not in the far future, and a sanity check against the zone bounds (drop absurd jumps). Drop frames arriving faster than ~3 s apart per order (server-side throttle) so a buggy client can't flood Redis.
3. **Write** `GEOADD` for dispatch (`riders:live`, with TTL semantics via a companion key or periodic cleanup — GEO members don't expire individually) and `PUBLISH` for the customer. Pub/sub is fire-and-forget; order status stays in Postgres.
4. **ETA:** do **not** call Directions on every ping. Recompute `eta_min` at most every 30–60 s per order (rider → current target: pickup before `picked_up`, drop after), via the cached `Router`; between recomputes, decrease the last ETA by elapsed time or reuse it. This keeps you far below 300 req/min even with many concurrent deliveries.
5. Close the room on `delivered` / `cancelled`; remove rider from the live geo set.
6. Heartbeat/ping-pong both directions; the customer app reconnects and should receive the **last known position** immediately (store last position per order in Redis, TTL ~1 h).
7. Don't log coordinates with identifiers beyond ids/status (your PII logging rule).

**Optional dispute trail (SRS FR-B04):** persist a sampled trail (e.g. 1 point / 30 s while active) in a new table `order_location_trail(order_id, lat, lng, recorded_at)` indexed on `(order_id, recorded_at)`; append-only. Add a migration only when you start working on disputes (Step 11/12).

---

## 6. Zones, service area, dispatch

- **Service-area check** (quote + create): haversine distance from zone center ≤ `service_radius_km` for pickup and drop. Pure function in `platform/geo`, table-tested.
- **Dispatch (Step 7):** keep Redis `GEOSEARCH` (straight-line, 1 → 3 → 5 km steps). Do **not** add the Matrix API for MVP. Later, if straight-line ranking picks the wrong rider (river/canal/rail barriers in Attock), re-rank only the top ≤ 5 candidates with Matrix (limits: 25 coords/request, 60 req/min — don't call it on every order burst without caching).
- **Admin live map (web):** needs a snapshot endpoint + WS feed, e.g. `GET /admin/live` returning active orders + online riders with coordinates. The admin web panel would use Mapbox GL JS (separate from this Flutter SDK; token and billing model differ — check Mapbox pricing before building).

---

## 7. Config & secrets (single typed struct, loaded in `cmd/api/main.go`)

```
MAPBOX_BACKEND_TOKEN=...           # secret, Render env var only
MAPBOX_BASE_URL=https://api.mapbox.com
MAPBOX_TIMEOUT_MS=3000
MAPBOX_ROUTE_CACHE_TTL=6h
MAPBOX_GEO_SEARCH_TTL=24h
MAPBOX_GEO_REVERSE_TTL=168h
ROUTE_CIRCUITY_FACTOR=1.3
ROUTE_FALLBACK_KMH=20
```

- Never log the token or full upstream URLs (they contain `access_token`). Wrap upstream errors, strip the query string before logging.
- Empty token → Mapbox features return "unavailable" (same pattern as the Firebase verifier in Step 17) and quotes fall back to haversine in dev.
- Two Mapbox tokens: **mobile public** (in the Flutter build) and **backend** (Render). Rotate independently.

---

## 8. Render deployment notes

- **WebSockets** are needed for live tracking; Render web services support them. Free-tier instances spin down after inactivity, which breaks live tracking and cold-starts the first quote — use a paid always-on instance before pilot.
- Redis: use the two-instance setup already described (cache vs asynq queue). Route/geo caches belong on the **cache** Redis (`allkeys-lru` is fine because every key has a TTL and is re-derivable).
- If you scale to >1 instance, pub/sub already fans out across instances; in-memory throttles/circuit-breaker counters should move to Redis keys with TTL.
- Add the Mapbox env vars in the Render dashboard; no code change needed per environment.

---

## 9. Observability & cost guard

- Counters (zerolog fields or metrics): `mapbox_directions_calls`, `mapbox_directions_cache_hits`, `mapbox_geocode_calls`, `mapbox_fallbacks`, `mapbox_429`.
- Log one line on every fallback with order/quote id (no coordinates tied to a person if you can avoid it).
- Alert on 429s and on fallback rate > a few %.
- Mapbox dashboard: review usage weekly the first month; set billing alerts.
- Expected load: one Directions call per **cache-missed** quote/create + one per ETA refresh (30–60 s per active order). Cache hit-rate on repeat pickup→drop pairs should be high for merchant-based orders.

---

## 10. Testing (per Backend Rules §9)

- Table-driven: haversine, service-area containment, cache-key rounding, quote fee math with a fake `Router` (success, rate-limited → fallback, no-route → 422).
- `mapbox.Client` tested with `httptest.Server`: 200, 401, 422, 429 (+ headers), timeout, malformed body.
- Geo handlers: input validation table (bad `q`, bad lat/lng, limit).
- WS: unauthorized rider/customer, frame too fast, bad coordinates, reconnect gets last position.
- No real Mapbox calls in unit tests.

---

## 11. Suggested build order (one change set each, `go test ./...` green)

1. `platform/geo` (haversine + zone containment) + tests.
2. `platform/mapbox` client + `Router` + `Geocoder` + `httptest` tests; typed config.
3. Migration for zone centers; extend `GET /zones`; contract test update.
4. Route cache + fallback + circuit-breaker light inside the orders pricing path; `GET /orders/quote`.
5. `POST /orders` recomputes distance server-side and persists it.
6. `internal/modules/geo`: `/geo/search`, `/geo/reverse` with caching + limits.
7. Live location payload contract, validation, last-position store, ETA refresh (Step 9).
8. (Later) location trail table, Matrix re-ranking, admin live snapshot.

---

## 12. Docs to update after this work

- **SRS FR-R05 / PRD / Tech Design:** Google Maps → Mapbox for maps; navigation = in-app route line + Google Maps handoff. Cost section: Maps SDK per-MAU, Directions/Geocoding per request.
- **Next_Steps.md:** add the steps above; record the locked decisions (server-computed distance, temporary geocoding + user-edited text, haversine fallback).
- **Backend_Rules.md §6:** no change needed (route/geo caches follow existing TTL rule).
