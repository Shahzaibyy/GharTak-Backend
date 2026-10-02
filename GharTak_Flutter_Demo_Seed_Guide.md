# GharTak — Demo seed & showcase guide (no SMS / no Firebase)

Goal: walk a client through the product using **seeded backend data** and **development OTP**, while Flutter only has UI shells. You do **not** need Twilio, Firebase, or a production Mapbox secret for a smooth happy path.

**Index:** `GharTak_Flutter_Integration_Index.md`.

---

## 1. What is already seeded (development)

When `APP_ENV=development`, the API ensures:

| Asset | How to use |
|---|---|
| 6 zones (2 active) | `GET /zones?active=true` → Attock + Hasan Abdal |
| Zone centers + 8 km radius | Migration `0008_zone_geo` |
| Demo merchant **approved** | ID `22222222-2222-4222-8222-222222222201`, Attock, menu Biryani + Kabab |
| Demo admin account | Phone `03000000002` |
| OTP without SMS | `dev_otp` on OTP request / phone-link request |

**Not seeded:** riders, fixed catalog UUIDs, fixed customer accounts, live orders. Catalog item IDs are created at seed time — always list the catalog.

---

## 2. OTP “bypass” — use `dev_otp`, do not skip auth

Skipping auth entirely would fight middleware and create fake showcase bugs. In development the backend **already bypasses SMS**:

```http
POST /auth/otp/request
Content-Type: application/json

{ "phone": "03001234567", "role": "customer" }
```

```json
{ "data": { "dev_otp": "482913" } }
```

```http
POST /auth/otp/verify
Content-Type: application/json

{ "phone": "03001234567", "role": "customer", "otp": "482913" }
```

Store `access_token` + `refresh_token`. Customer accounts are created on first verify with `phone_verified: true`, so **checkout is unblocked** without Google or phone-link.

### Flutter demo mode (recommended)

```dart
// Pseudocode — map to your existing env/flavor rules
if (Env.isDemo) {
  final req = await authApi.requestOtp(phone: demoPhone, role: 'customer');
  final otp = req.devOtp; // only present when API APP_ENV=development
  await authApi.verifyOtp(phone: demoPhone, role: 'customer', otp: otp!);
}
```

Optional UX: auto-fill the OTP field from `dev_otp` and hide the “resend SMS” copy in demo flavor. Keep the same screens you will use in production so the demo matches the real flow.

### Roles for showcase

| Role | Phone | Notes |
|---|---|---|
| Customer | any valid PK mobile, e.g. `03001234567` | Auto-created |
| Merchant | `03000000001` | Seed kitchen owner |
| Admin | `03000000002` | Seed platform admin |
| Rider | register first via `POST /riders/register`, then OTP | Not pre-seeded |

Google sign-in: leave **disabled** in demo builds until Firebase is configured (`POST /auth/google` → `unavailable` otherwise).

---

## 3. One-shot curl script (customer food order)

Replace `$API` and run against a migrated development API.

```bash
API="${API:-http://127.0.0.1:8080}"
PHONE=03001234567
ZONE=11111111-1111-4111-8111-111111111101
MERCHANT=22222222-2222-4222-8222-222222222201

OTP=$(curl -s "$API/auth/otp/request" -H 'Content-Type: application/json' \
  -d "{\"phone\":\"$PHONE\",\"role\":\"customer\"}" | jq -r .data.dev_otp)

TOKEN=$(curl -s "$API/auth/otp/verify" -H 'Content-Type: application/json' \
  -d "{\"phone\":\"$PHONE\",\"role\":\"customer\",\"otp\":\"$OTP\"}" | jq -r .data.access_token)

curl -s "$API/zones?active=true" | jq .
curl -s "$API/merchants?zone_id=$ZONE" | jq .
ITEM=$(curl -s "$API/merchants/$MERCHANT/catalog" | jq -r '.data[0].id')

curl -s "$API/orders/quote" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"type\":\"food\",\"zone_id\":\"$ZONE\",\"merchant_id\":\"$MERCHANT\",\"drop_lat\":33.78,\"drop_lng\":72.37,\"drop_address\":\"Demo drop, Attock\",\"items\":[{\"catalog_item_id\":\"$ITEM\",\"quantity\":1}],\"payment_method\":\"cod\"}" | jq .

curl -s "$API/orders" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"type\":\"food\",\"zone_id\":\"$ZONE\",\"merchant_id\":\"$MERCHANT\",\"drop_lat\":33.78,\"drop_lng\":72.37,\"drop_address\":\"Demo drop, Attock\",\"items\":[{\"catalog_item_id\":\"$ITEM\",\"quantity\":1}],\"payment_method\":\"cod\",\"client_request_id\":\"demo-order-0001\"}" | jq .
```

Pin stays inside Attock’s 8 km circle around `33.7667, 72.3667` or the API returns invalid/out-of-area.

---

## 4. What Flutter should hardcode for demos (safe)

Hardcode **only** these constants in a `DemoConfig` (flavor-gated):

```text
attockZoneId   = 11111111-1111-4111-8111-111111111101
demoMerchantId = 22222222-2222-4222-8222-222222222201
demoCustomerPhone = 03001234567
demoMerchantPhone = 03000000001
demoAdminPhone    = 03000000002
```

Do **not** hardcode catalog item IDs or access tokens in source control.

---

## 5. Gaps that can still break a live demo

| Gap | Mitigation |
|---|---|
| No seeded rider → no accept / live track | Demo stops at “order placed”, or pre-register + admin-approve a rider before the meeting |
| Backend Mapbox token missing | Quotes still work (`approximate: true`); disable address search UI |
| Redis down | OTP and sessions fail — verify `/health` and Redis before the meeting |
| Wrong `APP_ENV` | No `dev_otp` outside development — demo flavor must hit a development API |
| Rate limits (5 OTP/hour/phone) | Rotate demo phones (`03001234568`…) if you rehearse often |
| Idempotent place | Reuse same `client_request_id` only when you want the same order back |

---

## 6. Optional later backend improvement (not required for FE wiring)

If client demos need a full rider loop repeatedly, add a **development-only** seed for:

- one approved rider (fixed phone),
- fixed catalog item UUIDs,
- optional sample `delivered` order for history UI.

Until then, the existing merchant + zone + `dev_otp` path is enough for catalog, map camera, quote, and place.

---

## 7. FE integration order for a stable showcase

1. Wire Dio + token storage + envelope errors.
2. Auth with auto `dev_otp` in demo flavor ([`GharTak_Flutter_API_Auth.md`](./GharTak_Flutter_API_Auth.md)).
3. Zones → merchants → catalog → quote → place ([`GharTak_Flutter_API_Catalog_Orders.md`](./GharTak_Flutter_API_Catalog_Orders.md)).
4. Map camera + optional geo ([`GharTak_Flutter_Mapbox_Guide.md`](./GharTak_Flutter_Mapbox_Guide.md)).
5. Leave Google, FCM, S3 uploads, and rider live map for a second demo pass.
