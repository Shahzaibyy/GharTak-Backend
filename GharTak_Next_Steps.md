# GharTak Backend — MVP implementation sequence

Start here after the foundation in this repo. Do one step per change set. Keep `go test ./...` green before moving on. Auth and onboarding completion is Step 14 onward.

Flutter clients wiring these APIs: start at `GharTak_Flutter_Integration_Index.md` (demo seed + auth + catalog/orders + Mapbox).

The product docs stay the source of truth for behavior: `GharTak_PRD.md`, `GharTak_SRS.md`, `GharTak_Technical_Design.md`. `GharTak_Backend_Rules.md` is the engineering constraint. This file is the order of work so later steps extend the schema and types already here.

## What is already in place

- Chi router, config, zerolog, pgx pool, and a SQL migration runner.
- `migrations/0001_init.up.sql` and `migrations/0002_seed_zones.up.sql`.
- Phone normalization and AES-GCM plus HMAC lookup in `internal/platform/pii`.
- PKR money as integer paisa, JSON decimal strings, in `internal/platform/money`.
- Order status machine in `internal/modules/orders`.
- Public `GET /health` and `GET /zones`.
- A contract test that fails if Go constants and the SQL checks drift apart.

Run it from the repo root:

```bash
cp .env.example .env
docker compose up -d
go run ./cmd/api
curl -s localhost:8080/health
curl -s localhost:8080/zones
curl -s 'localhost:8080/zones?active=true'
```

`GET /zones?active=true` returns Attock City and Hasan Abdal. Those two are the Phase 1 launch cities. The other four tehsils are seeded and inactive.

## Locked decisions

These are already encoded in the schema, Go constants, or both. A later step that needs a new value adds a new migration and updates the matching Go list. The contract test in `internal/platform/schema/contract_test.go` must stay green.

| Topic | Decision |
|---|---|
| Router | `chi` only. |
| Layout | `handler → service → repository`. SQL lives in the repository file of that module. |
| JSON | `snake_case`. Errors are `{"error":{"code","message"}}`. Success is `{"data":...}`. |
| Money | `NUMERIC(14,2)` in Postgres, paisa in Go, decimal strings in JSON. No `float64`. |
| Phone and CNIC | Ciphertext plus lowercase HMAC-SHA256 hex lookup. Use `pii.SealPhone`. |
| OTP and refresh tokens | Redis with a TTL. Postgres stays the source of truth for accounts and money. |
| Order status | Only `orders.Transition`. Re-offering a rider does not change status; it stays `rider_offered`. |
| Offers | Rows in `order_offers`. The first `accepted` row wins. `rider_id` is set on the order at that transition. |
| Cart | The client holds the cart until `POST /orders`. |
| Photos and CNIC files | Store object keys (`photo_key`, `cnic_object_key`). Uploads go through presigned URLs. The API body limit stays 1 MB. |
| Server-computed distance | Quote and place use Mapbox Directions (cached). Client never sends `distance_km`. On Mapbox failure, haversine × `ROUTE_CIRCUITY_FACTOR` with `approximate=true`. |
| Geocoding storage | Temporary geocoding only. Persist map-pin lat/lng + user-edited address text (approach A). |
| Zone geometry | `zones.center_lat` / `center_lng` + `service_radius_km` for camera lock and out-of-area checks. |
| Orders | Cancel them. Leave the row in place. `order_events`, `ledger_entries`, `ratings`, and `blacklist` are append-only. |
| Catalog delete | Allowed. `order_items` keeps `item_name` and `price_at_order`. |
| Merchant hours | A later migration adds `weekly_hours`. |
| Delivery fee split | Rider receives 80% (`payments.RiderDeliveryShareBPS = 8000`). Platform keeps 20%. Commission is a percent of `item_total` only. Surge multiplies the delivery fee before that split. |
| Commission defaults | Restaurant `18.00`, mart `10.00`, pharmacy `10.00`. `merchants.DefaultCommissionPercent`. |
| Errand and courier price | Zone base + per km, then effort (`1.00` / `1.30` / `1.60`), then surge. `item_total` stays `0`. |
| Food and mart price | `item_total` plus delivery fee. `merchant_id` is required. Courier and errand keep `merchant_id` null. |
| Wallet and cash owed | Cache columns. `ledger_entries` is authoritative. Update the cache in the same transaction as the ledger insert, with `SELECT ... FOR UPDATE`. |
| Platform ledger account | `payments.PlatformAccountID`. Escrow `account_id` is the order id. |
| Webhooks | One `payments` row per order. `gateway_ref` is unique. Retries no-op when that ref already exists. |
| Live location | Mount the WebSocket on the root mux, outside the 15s HTTP timeout group in `cmd/api/main.go`. |
| Rate limit IP | `RemoteAddr` with the port stripped. Turn on forwarded-header trust only behind a known proxy. |
| Google sign-in | Customer role only. Firebase Authentication, verified with the Admin SDK `VerifyIDToken`. The client then holds GharTak access and refresh JWTs. Auth middleware still checks only those JWTs. |
| Google and phone accounts | Never merged. A Google signup has a null phone until `POST /auth/phone/link`. A later phone-OTP verify still creates its own user row. |
| Phone before checkout | `POST /orders` returns `409 {"error":{"code":"phone_required",...}}` when `phone_verified` is false. Quote stays available. |
| Account deletion | Soft delete. `deleted_at` is set. Name becomes `Deleted`. Email, phone ciphertext, phone lookup, and `firebase_uid` are cleared so the format check and unique indexes stay valid. Wallet must be zero. Order rows stay. |
| Uploads | `POST /uploads/presign` returns a 15 minute PUT URL, `object_key`, `content_type` (`image/jpeg`), and `max_bytes`. CNIC and selfie are 2 MB. Vehicle documents are 5 MB. Catalog and proof photos are 8 MB. |
| Phone link body | `{phone}` then `{otp}`. The completion note's empty link body cannot name a number, so the phone is required on the first call and held in Redis until verify. |

### Ledger direction

Credit increases a liability, wallet, or income balance. Debit decreases it. For assets (`rider_cash_owed`, `gateway_clearing`, `settlement_cash`), debit increases the balance and credit decreases it. Every journal's debit total equals its credit total. Skip a leg whose amount is zero. `idempotency_key` is unique per leg, and every leg in one journal shares `journal_id`.

| Event | Legs |
|---|---|
| Wallet checkout | Debit `customer_wallet`. Credit `escrow` (order id). Type `escrow_hold`. |
| JazzCash or Easypaisa checkout | Debit `gateway_clearing` (platform id). Credit `escrow`. Type `escrow_hold`. |
| Digital delivery | Debit `escrow` for item total + delivery fee. Credit `merchant_payable` for item total minus commission when that is positive. Credit `rider_earnings` for `rider_earning`. Credit `platform_revenue` for the remainder. Type `escrow_release`. |
| Wallet refund before delivery | Debit `escrow`. Credit `customer_wallet`. Type `refund`. |
| Gateway refund before delivery | Debit `escrow`. Credit `gateway_clearing`. Type `refund`. |
| COD placement | No ledger rows. Payment row is `method=cod`, `status=held`. |
| COD delivery | Debit `rider_cash_owed` for the remittance (`item_total + delivery_fee - rider_earning`). Credit `merchant_payable` for item total minus commission when positive. Credit `platform_revenue` for the remainder. Do not credit `rider_earnings`; the rider already kept that cash. Type `cod_recognized`. |
| COD settlement | Credit `rider_cash_owed`. Debit `settlement_cash`. Type `cod_settled`. |
| Admin wallet credit | Debit `adjustment` (platform id). Credit `customer_wallet`. Type `wallet_adjustment`. |

COD cancel after delivery is a dispute flow. It is outside the delivery step.

## Schema rules for every later step

- Add `migrations/000N_name.up.sql` and a matching `.down.sql`. Leave `0001` and `0002` unchanged.
- Name every column in new queries. Parameterize every value.
- Index every new foreign key. Match composite and partial indexes to the real `WHERE` clause.
- Run `EXPLAIN (ANALYZE, BUFFERS)` on every new query before the step is done. A sequential scan is acceptable on `zones` because that table stays at district size. Flag a sequential scan on any table expected to grow past a few thousand rows.
- Multi-step writes share one `pgx.Tx`.
- Pass `context` with `database.WithTimeout`.
- Map domain errors through `internal/platform/httpx.WriteError`. Add a sentinel to that catalog when a new HTTP status is required.
- Keep each function at or under 4 branch points. Use a map dispatch where a chain of order-type or payment-method checks would otherwise appear.
- Logs may contain ids, status, and paths. They omit phone numbers, CNIC, OTP, and payment tokens.

The only query shipped with this foundation is the zone list. `EXPLAIN (ANALYZE, BUFFERS)` on the seeded table uses an index scan on `zones_city_name_unique` (`ORDER BY city_name`), returns 6 rows, and finishes in about 0.1 ms. That index exists because city names are unique. It is the right plan for this bounded list.

## Step 1 — Auth

Module: `internal/modules/auth`.

- `POST /auth/otp/request` with `{phone, role}`. `role` is `customer`, `rider`, `merchant`, or `admin`. The same phone may exist on more than one side.
- `POST /auth/otp/verify` returns a short-lived access JWT (about 1 hour) and a refresh token.
- Store the OTP in Redis at `otp:{role}:{phone_lookup}` with a 5 minute TTL. Hash the OTP before storage.
- Add a stricter limiter on these two routes: 5 requests per hour per phone lookup and 20 per hour per IP, in addition to the global limiter.
- Customer first verify creates the `users` row via `pii.SealPhone`. Rider, merchant, and admin rows are created by their own onboarding steps and stay inactive until approval.
- Refresh tokens live in Redis, rotate on use, and are revoked when reused.
- Middleware reads the bearer token, checks the signature, and puts account id plus role on the request context. Handlers do not parse JWTs themselves.
- Table-driven tests cover the verify outcomes: match, wrong code, expired, wrong role. The test uses a fake OTP store.

## Step 2 — Merchant catalog

Module: `internal/modules/merchants`.

- Merchant registration writes `merchants` with the category commission from `DefaultCommissionPercent`, zone id, pin, and encrypted phone. `verification_status` stays `pending`.
- Admin approval is a later admin step. Until then, a dev-only seed merchant is enough to exercise the menu.
- Catalog CRUD uses `photo_key` and the availability flag. Customer `GET /merchants?zone_id=` returns approved merchants in that zone. `GET /merchants/{id}/catalog` returns available items, with a fixed limit.
- Index use: `idx_merchants_browse` and `idx_catalog_available`.

## Step 3 — Addresses

Still in the customer/auth side, repository under a customer-facing package that owns `addresses`.

- Authenticated CRUD for the caller's own rows.
- `POST /orders` will copy lat, lng, and text onto the order. It will not keep a foreign key to `addresses`, so later edits do not rewrite history.

## Step 4 — Place an order

Module: `internal/modules/orders`.

- `POST /orders` validates the body once in the handler. The service trusts it.
- One transaction inserts `orders`, `order_items` for food and mart, the first `order_events` row (`to_status=placed`, `actor_role=customer`), and a `payments` row with status `held`.
- Call `orders.RequiresMerchant` and `orders.Transition` rather than new status strings.
- Require `client_request_id`. The unique index makes a retry return the original order.
- Food and mart start at `placed`. Courier and errand also start at `placed`; dispatch moves them to `rider_offered`.
- This step does not call Redis, FCM, or a payment gateway.

## Step 5 — Pricing

Pure function in the orders module, table-driven, no database — plus Mapbox road distance.

- Load the zone once, then compute delivery fee from `base_delivery_fee`, `per_km_rate`, **server road distance**, effort basis points, and `surge_multiplier`.
- `internal/platform/mapbox` Directions (cached 6h on cache Redis) with haversine fallback. Quote returns `duration_min`, `approximate`, and optional GeoJSON `route`.
- Pickup and drop must fall inside the zone radius around `center_lat`/`center_lng`.
- Add `money.MulBPS` here, with half-up rounding to the nearest paisa, and tests for 0.5 paisa boundaries.
- Persist `distance_km`, `delivery_fee`, `commission_amount`, `rider_earning`, and `surge_multiplier` on the order inside the Step 4 transaction.
- `POST /orders/quote` is the fee preview (same body shape as place, without requiring payment). `GET /geo/search` and `GET /geo/reverse` proxy Geocoding v6 with `country=pk`.

## Step 6 — Ledger

Module: `internal/modules/payments`.

- Implement the journals in the table above and nothing else.
- Wallet checkout checks `wallet_balance` with `FOR UPDATE` before the debit.
- COD does not write ledger rows until delivery.
- Gateway capture can be a stub that records `gateway_ref` and the escrow journal. Real JazzCash and Easypaisa clients come after the journal tests are green.
- Webhook handlers look up `gateway_ref` first and return success when the payment is already in the target status.

## Step 7 — Dispatch

Module: `internal/modules/dispatch`.

- Redis `GEOADD` for online approved riders. Key per zone, TTL on the position.
- On `ready_for_pickup` (food and mart) or `placed` (courier and errand), transition to `rider_offered` and insert `order_offers`.
- First accept updates the offer to `accepted` and the order to `accepted` with `rider_id` in one transaction. The conditional update is `WHERE status = 'rider_offered' AND rider_id IS NULL`.
- Reject and expiry update the offer row only. A new offer to the next rider stays on status `rider_offered`.
- Radius steps are 1 km, then 3 km, then 5 km, capped by the zone `service_radius_km`.

## Step 8 — Rider task updates

- `POST /riders/tasks/{id}/accept|reject|pickup|deliver` calls `orders.Transition` for that order type.
- Deliver checks a hashed customer OTP, or stores `proof_photo_key` for courier and errand.
- The deliver transaction also runs the Step 6 release or COD journal and sets `delivered_at`.
- Rider online toggle updates `is_online` only when `riders_online_ready` would pass: approved and zoned.

## Step 9 — Live location

- `WS /ws/location/{order_id}` on the root mux, outside HTTP timeout.
- Rider publishes every 5 to 10 seconds. Redis pub/sub `order:{id}:location` fans out to the customer socket.
- Pub/sub is fire-and-forget. Order status stays in Postgres.

## Step 10 — Notifications

Module: `internal/modules/notifications`.

- FCM on each status change. The order service calls a small notifier interface.
- Templates live in this module. Failures are logged and do not roll back the order transaction.

## Step 11 — Ratings and tickets

- Ratings insert through the append-only `ratings` table after `delivered`.
- Recompute `riders.rating` and `rating_count` in that same transaction.
- Support tickets are a new migration, with `order_id`, status, and messages. That migration is the first schema change after this foundation.

## Step 12 — Admin, fraud, payouts

Module: `internal/modules/admin`, plus a scheduled job.

- Approve or suspend riders, merchants, and customers. Approval sets rider `zone_id` and `verification_status` together.
- Zone pricing updates `zones` columns inside the existing checks (surge at most 3, radius at most 50 km).
- Fraud job counts delivered pairs and inserts one open `fraud_flags` row when the pair crosses the threshold. The partial unique index blocks duplicates.
- Merchant payout reads `merchant_payable` from the ledger. It does not add a second balance column until a profile shows the sum is too slow.

## Background jobs and events (implemented)

- **Cache Redis** (`REDIS_URL`): OTP, refresh tokens, rider geo, live location and chat pub/sub. Local compose uses `allkeys-lru` and no AOF.
- **Queue Redis** (`REDIS_QUEUE_URL`): asynq only. Local compose service `redis-queue` on port `6380` with AOF and `noeviction`. Production should use a separate Upstash/Render instance with the same policy.
- **Worker**: runs inside the API when `RUN_ASYNQ_WORKER=true`, or as `go run ./cmd/worker` with the same env. Handles FCM (`low` queue, retries), rider offer timeout re-dispatch (`critical`, delayed 30s, unique per order), and nightly fraud scan cron (stub until Step 12 fraud job lands).
- **NATS** (optional `NATS_URL`): JetStream stream `GHARTAK`, subject `ghartak.events.order.status`. API publishes; worker durable consumer enqueues notification tasks. Without NATS, events enqueue asynq directly.
- **asynqmon**: admin-only UI at `/admin/queue/` (Bearer admin JWT).

## Step 13 — Hardening

- `govulncheck` in CI.
- Per-route limits already started in Step 1; confirm OTP and order create.
- Masking for in-app calls is an integration behind an interface. Real numbers stay off the payload to the other party.
- Nightly Postgres dump to object storage.

## Step 14 — Profile schema

Migration `migrations/0007_auth_google_and_profile.up.sql` and its down file. Leave `0001` through `0006` unchanged.

- `users.phone_ciphertext` and `users.phone_lookup` become nullable.
- Add `email`, `firebase_uid`, `phone_verified`, and `deleted_at`.
- Replace `users_phone_lookup_unique` with a partial unique index `WHERE phone_lookup IS NOT NULL`.
- Add `users_firebase_uid_unique` on `firebase_uid` where it is not null.
- Backfill `phone_verified = true` for every row that already has a phone lookup.
- Customer OTP insert sets `phone_verified = true`. Google insert leaves it false.
- `ON CONFLICT` targets must include the partial-index predicate.

## Step 15 — Profile

`GET /users/me` and `PATCH /users/me` on the customer role.

- Response is `{id, name, email, phone, phone_verified, wallet_balance}`. Phone is null when the row has no ciphertext. Wallet is a decimal string.
- Decrypt the phone only for this caller. Do not log it.
- Patch accepts `{name}` once, 1 to 100 characters. The service trusts that.

## Step 16 — Presigned uploads

`internal/platform/uploads`. `POST /uploads/presign` for any authenticated role.

- Body `{purpose}` is one of `cnic`, `selfie`, `vehicle_doc`, `catalog_photo`, `proof_of_delivery`. Dispatch with the purpose map, not an if-else chain.
- Object key is `{prefix}/{account_id}/{uuid}`.
- When `S3_ACCESS_KEY` and `S3_SECRET_KEY` are set, the URL is a path-style SigV4 PUT signed for `Content-Type: image/jpeg`. Otherwise development returns an unsigned URL on `S3_ENDPOINT`.
- Rider, merchant, and proof screens send `object_key` on the owning request. Rider OTP still requires an existing rider row, so a first-time rider registers before a later document replace, or uploads after a token exists.

## Step 17 — Google sign-in

`POST /auth/google` with `{firebase_id_token}`.

- Inject the Firebase verifier from `cmd/api`, next to the pool and Redis client. Empty credentials make the route return unavailable. Do not construct the client inside the handler.
- Verify the token, look up `users` by `firebase_uid`, and create a customer row when none exists. Do not match email or phone.
- Copy `name` and `email` from the token when they fit. Issue the same session shape as OTP verify, role `customer`.
- Table-driven cases: new user, returning user, expired token, wrong audience.

## Step 18 — Phone link and checkout gate

- `POST /auth/phone/link` with `{phone}` stores a hashed OTP and the sealed phone in Redis for 5 minutes, under the current user. Same limiter as OTP request.
- `POST /auth/phone/link/verify` with `{otp}` writes `phone_ciphertext`, `phone_lookup`, and `phone_verified = true` on that row. A lookup that already belongs to someone else is a conflict.
- `POST /orders` calls `RequirePhone` before insert. An existing `client_request_id` still returns the original order. A missing phone returns `phone_required` and does not insert.

## Step 19 — Logout and account deletion

- `POST /auth/logout` with `{refresh_token}` deletes that refresh key immediately and returns 204. It does not wait for reuse detection. A missing token is still 204.
- `DELETE /users/me` returns 204. Lock the user row, refuse a non-zero wallet, then scrub the identity columns and set `deleted_at`. Clearing `firebase_uid` lets a later Google sign-in create a new row.

## Step 20 — Merchant approval

Same shape as rider approve and reject. Only a `pending` merchant moves.

- `POST /admin/merchants/{id}/approve`
- `POST /admin/merchants/{id}/reject`

The status write stays in `internal/modules/merchants`. Admin routes call it. Customer browse already hides unapproved merchants.

## Module map

| Path | Owns |
|---|---|
| `internal/modules/auth` | OTP, Google sign-in, JWT, refresh, profile, phone link, logout, account deletion |
| `internal/platform/uploads` | Presigned upload URLs |
| `internal/modules/orders` | Status machine, pricing, order writes |
| `internal/modules/merchants` | Merchant profile and catalog |
| `internal/modules/dispatch` | Offers and Redis geo |
| `internal/modules/payments` | Ledger journals and webhooks |
| `internal/modules/notifications` | FCM |
| `internal/modules/admin` | Zones, verification, fraud queue |

`internal/modules/merchants` is an addition to the folder list in the engineering rules. The technical design has a merchant module, and catalog data cannot live cleanly inside admin. Cross-module calls go through the exported interfaces of these packages.

## Seed zone ids

| City | Id | Active |
|---|---|---|
| Attock City | `11111111-1111-4111-8111-111111111101` | yes |
| Hasan Abdal | `11111111-1111-4111-8111-111111111102` | yes |
| Hazro | `11111111-1111-4111-8111-111111111103` | no |
| Fateh Jang | `11111111-1111-4111-8111-111111111104` | no |
| Jand | `11111111-1111-4111-8111-111111111105` | no |
| Pindi Gheb | `11111111-1111-4111-8111-111111111106` | no |

Base fee `60.00`, per km `18.00`, surge `1.00`, radius `8.00`. Admin pricing changes these rows later. It does not insert a second set of cities.
