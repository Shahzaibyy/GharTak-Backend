# GharTak — Flutter ↔ Backend integration index

Use these guides while the Flutter UI is already built and APIs are not wired yet. Copy them into the Flutter repo docs folder if you prefer them next to your app rules; they stay in this backend repo so endpoint contracts match `cmd/api/openapi.yaml`.

**Source of truth for HTTP shapes:** `cmd/api/openapi.yaml` + Swagger at `/docs/` when the API is running.  
**Backend constraints:** `GharTak_Backend_Rules.md`.  
**Product behavior:** `GharTak_PRD.md`, `GharTak_SRS.md`.

---

## Guides in this set

| File | Use when |
|---|---|
| [`GharTak_Flutter_Demo_Seed_Guide.md`](./GharTak_Flutter_Demo_Seed_Guide.md) | Client demo / QA without SMS, Firebase, or Mapbox billing drama |
| [`GharTak_Flutter_Mapbox_Guide.md`](./GharTak_Flutter_Mapbox_Guide.md) | Maps SDK on device + which calls must hit **this** backend |
| [`GharTak_Flutter_API_Auth.md`](./GharTak_Flutter_API_Auth.md) | OTP, session, profile, phone link (no SMS provider needed in `development`) |
| [`GharTak_Flutter_API_Onboarding.md`](./GharTak_Flutter_API_Onboarding.md) | Customer + rider HTML onboarding screens → endpoints |
| [`GharTak_Flutter_API_Catalog_Orders.md`](./GharTak_Flutter_API_Catalog_Orders.md) | Zones → merchants → menu → quote → place → track |

Integrate in that order: **demo seed → auth → onboarding → catalog/orders → Mapbox/geo**. Do not wire Mapbox pricing on the phone.

---

## Backend dependencies the Flutter app needs

| Dependency | Required for showcase? | Notes |
|---|---|---|
| Running API (`APP_ENV=development`) | Yes | Local or Render. Base URL in Flutter flavors. |
| Postgres + migrations through `0008_zone_geo` | Yes | Zones + centers. |
| Redis cache (`REDIS_URL`) | Yes | OTP, sessions, geo/route cache. |
| Redis queue (`REDIS_QUEUE_URL`) | Soft | Needed for background jobs; quoting still works without a worker for demo. |
| `MAPBOX_BACKEND_TOKEN` on **server** | Recommended | Empty → quotes work with `approximate: true`; `/geo/*` returns unavailable. |
| Flutter Mapbox **public** token (Maps SDK) | Yes for map UI | Display-only. Never put the **backend** Directions/Geocoding secret in the app. |
| SMS / OTP gateway | **No** in development | `POST /auth/otp/request` returns `dev_otp`. |
| Firebase Admin on backend | **No** for phone OTP demo | Only needed for `POST /auth/google`. Skip Google button in demo builds. |
| S3 / MinIO | Soft | Presign returns a local-style URL when keys are empty. |
| FCM | Soft | Notifications log when `FCM_SERVER_KEY` is empty. |

---

## Flutter layering (map to your existing rules)

Keep UI free of Dio/http and JSON keys. Suggested mapping to a typical Flutter structure:

```
lib/
  core/           # Env, Dio client, interceptors, Result/Failure, tokens
  features/
    auth/
    zones/
    merchants/
    orders/
    geo/          # search/reverse DTOs + repository (HTTP only)
    map/          # Mapbox SDK widgets; consumes Zone + Quote.route
```

Rules that avoid showcase failures:

1. **One Dio client** with `Authorization: Bearer` interceptor and 401 → refresh → retry once.
2. **Envelope parsing:** success `data`, error `error.code` / `error.message`. Map `phone_required`, `rate_limited`, `unavailable` explicitly.
3. **Money as `String`** (decimal), never `double`.
4. **Cart stays on device** until `POST /orders`.
5. **Never send `distance_km`.** Server prices from lat/lng.
6. **Geocoder results are temporary.** Persist pin lat/lng + user-edited address text.
7. Feature flags: `useDevOtpAutoFill`, `enableGoogleSignIn`, `enableMapboxGeo` so a demo build can soft-fail geo when the backend token is missing.

---

## Stable seed IDs (development)

| Resource | ID / phone |
|---|---|
| Attock City zone | `11111111-1111-4111-8111-111111111101` |
| Hasan Abdal zone | `11111111-1111-4111-8111-111111111102` |
| Fateh Jang zone (active in demo) | `11111111-1111-4111-8111-111111111104` |
| Demo restaurant (Attock) | `22222222-2222-4222-8222-222222222201` · phone `03000000001` |
| Fateh Jang restaurants | phones `03003333001`–`03003333008` (see Demo Seed guide) |
| Fateh Jang customers | phones `03001111001`–`03001111005` |
| Fateh Jang riders (online) | phones `03002222001`–`03002222005` |
| Demo admin | phone `03000000002` (UUID assigned at seed) |
| Catalog item IDs | **Not fixed** — always `GET /merchants/{id}/catalog` |

---

## Showcase happy path (customer food order)

1. `POST /auth/otp/request` + `verify` with `dev_otp` → store tokens.
2. `GET /zones?active=true` → lock map camera to Attock center + 8 km radius.
3. `GET /merchants?zone_id=…101` → open demo kitchen.
4. `GET /merchants/…201/catalog` → pick Biryani / Kabab IDs.
5. Drop pin inside zone → optional `GET /geo/reverse` for label hint → user edits address.
6. `POST /orders/quote` → show fees + draw `route` if present.
7. `POST /orders` with `client_request_id` + `payment_method: cod`.
8. Poll `GET /orders/{id}` (rider live map needs an approved rider — optional for UI-only demo).

Full curl recipes: [`GharTak_Flutter_Demo_Seed_Guide.md`](./GharTak_Flutter_Demo_Seed_Guide.md).
