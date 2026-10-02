# GharTak — Demo seed & showcase guide (no SMS / no Firebase)

Goal: walk a client through the product using **seeded backend data** and **development OTP**, while Flutter only has UI shells. You do **not** need Twilio, Firebase, or a production Mapbox secret for a smooth happy path.

**Index:** `GharTak_Flutter_Integration_Index.md`.

---

## 1. What is already seeded (development)

When `APP_ENV=development`, the API ensures (idempotent on each boot):

| Asset | How to use |
|---|---|
| Zones | `GET /zones?active=true` → Attock, Hasan Abdal, **Fateh Jang** (migration `0010`) |
| Zone centers + 8 km radius | Migration `0008_zone_geo` — Fateh Jang center `33.5672, 72.6417` |
| Demo kitchen (Attock) | ID `…2201`, phone `03000000001` |
| **8 Fateh Jang restaurants** | Real map pins + menus — phones `03003333001`–`008` |
| **5 Fateh Jang customers** | Named users + home addresses — phones `03001111001`–`005` |
| **5 Fateh Jang riders** | Approved, oriented, **online** near restaurants — phones `03002222001`–`005` |
| Demo admin | Phone `03000000002` |
| OTP without SMS | `dev_otp` on OTP request / phone-link / email request |

Catalog item IDs are generated at seed — always `GET /merchants/{id}/catalog`.

### Fateh Jang merchants (coordinates from public map listings)

| Phone | Name | Approx lat,lng |
|---|---|---|
| `03003333001` | Bismillah Restaurant | 33.567565, 72.641856 |
| `03003333002` | Talha Hotel | 33.565425, 72.641523 |
| `03003333003` | Spanish Pizza Fateh Jang | 33.570230, 72.646564 |
| `03003333004` | Ramzan Hotel & Fast Foods | 33.570048, 72.646179 |
| `03003333005` | Sawat Naan Centre | 33.571450, 72.648878 |
| `03003333006` | Mehria Hotel & Restaurant | 33.578310, 72.653180 |
| `03003333007` | Ramzan Murgh Pulao | 33.567918, 72.643240 |
| `03003333008` | Abaseen Restaurant | 33.593820, 72.698970 |

### Fateh Jang login phones

| Role | Phones |
|---|---|
| Customer | `03001111001` Ayesha · `002` Ali · `003` Sana · `004` Omar · `005` Hira |
| Rider | `03002222001` Usman · `002` Bilal · `003` Hamza · `004` Saad · `005` Farhan |
| Merchant | `03003333001`–`008` (match restaurant table) |

Zone id for Flutter: `11111111-1111-4111-8111-111111111104`.

---

## 2. Demo login portal (no OTP)

Skip the OTP screens while testing. With `APP_ENV=development`:

1. Open **`https://YOUR-API/demo/`** in a browser  
2. Tap a seeded customer / rider / merchant / admin  
3. Access token is copied — paste into Flutter secure storage or Swagger **Authorize**

APIs (same thing for the app):

```http
GET  /auth/demo/accounts
POST /auth/demo/login
{ "phone": "03001111001", "role": "customer" }
```

Returns a normal session (`access_token`, `refresh_token`, …). **404 outside development.**

Flutter demo flavor can call `/auth/demo/login` instead of OTP request/verify.

### Still available: OTP with `dev_otp`

If you prefer the real OTP screens in the app, request OTP and use `dev_otp` from the response (no SMS):

```http
POST /auth/otp/request
Content-Type: application/json

{ "phone": "03001111001", "role": "customer" }
```

```json
{ "data": { "dev_otp": "482913" } }
```

```http
POST /auth/otp/verify
Content-Type: application/json

{ "phone": "03001111001", "role": "customer", "otp": "482913" }
```

Store `access_token` + `refresh_token`.

### Flutter demo mode (recommended)

Prefer one-tap demo login (no OTP):

```dart
if (Env.isDemo) {
  final session = await authApi.demoLogin(phone: '03001111001', role: 'customer');
  await tokenStore.save(session);
}
```

Or keep OTP screens and auto-fill `dev_otp` from `/auth/otp/request` in development.

### Roles for showcase

| Role | Phone | Notes |
|---|---|---|
| Customer (Fateh Jang) | `03001111001` … `005` | Pre-seeded name + home address |
| Customer (any new) | e.g. `03001234567` | Auto-created on first verify |
| Merchant (Fateh Jang) | `03003333001` … `008` | Approved restaurants |
| Merchant (Attock demo) | `03000000001` | Original demo kitchen |
| Admin | `03000000002` | Seed platform admin |
| Rider (Fateh Jang) | `03002222001` … `005` | Approved + online in zone |

Google sign-in: leave **disabled** in demo builds until Firebase is configured (`POST /auth/google` → `unavailable` otherwise).

### Fateh Jang smoke curl

```bash
API="${API:-http://127.0.0.1:8080}"
ZONE=11111111-1111-4111-8111-111111111104
PHONE=03001111001

OTP=$(curl -s "$API/auth/otp/request" -H 'Content-Type: application/json' \
  -d "{\"phone\":\"$PHONE\",\"role\":\"customer\"}" | jq -r .data.dev_otp)
TOKEN=$(curl -s "$API/auth/otp/verify" -H 'Content-Type: application/json' \
  -d "{\"phone\":\"$PHONE\",\"role\":\"customer\",\"otp\":\"$OTP\"}" | jq -r .data.access_token)

curl -s "$API/zones?active=true" | jq '.data[] | {city_name,id,center_lat,center_lng}'
curl -s "$API/merchants?zone_id=$ZONE" | jq '.data[] | {name,lat,lng}'
```

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

Richer demo history (sample delivered orders) can be added later. Fateh Jang merchants, customers, and online riders are already seeded in development.

---

## 7. FE integration order for a stable showcase

1. Wire Dio + token storage + envelope errors.
2. Auth with auto `dev_otp` in demo flavor ([`GharTak_Flutter_API_Auth.md`](./GharTak_Flutter_API_Auth.md)).
3. Zones → merchants → catalog → quote → place ([`GharTak_Flutter_API_Catalog_Orders.md`](./GharTak_Flutter_API_Catalog_Orders.md)).
4. Map camera + optional geo ([`GharTak_Flutter_Mapbox_Guide.md`](./GharTak_Flutter_Mapbox_Guide.md)).
5. Leave Google, FCM, S3 uploads, and rider live map for a second demo pass.
