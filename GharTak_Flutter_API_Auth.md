# GharTak — Flutter API integration: Auth & onboarding

**Index:** `GharTak_Flutter_Integration_Index.md`  
**Demo without SMS:** `GharTak_Flutter_Demo_Seed_Guide.md`  
**Contract:** `cmd/api/openapi.yaml`

Wire this feature first. All other authenticated calls depend on it.

---

## 1. Flutter placement (align to your rules)

```
features/auth/
  data/          # DTOs, AuthApi (Dio), AuthRepositoryImpl, token store
  domain/        # entities, AuthRepository, use cases
  presentation/  # already-built UI; ViewModels / Blocs / Notifiers only
```

- UI talks to domain only.
- Persist tokens in your existing secure storage.
- Single session source of truth; clear on logout / soft-delete.

---

## 2. Endpoints

| Method | Path | Auth | Demo ready? |
|---|---|---|---|
| `POST` | `/auth/otp/request` | No | Yes — returns `dev_otp` when API `APP_ENV=development` |
| `POST` | `/auth/otp/verify` | No | Yes |
| `POST` | `/auth/refresh` | No | Yes |
| `POST` | `/auth/logout` | No | Yes |
| `POST` | `/auth/google` | No | **No** until Firebase + backend credentials |
| `POST` | `/auth/phone/link` | Bearer customer | Yes (`dev_otp`) |
| `POST` | `/auth/phone/link/verify` | Bearer customer | Yes |
| `GET` | `/users/me` | Bearer customer | Yes |
| `PATCH` | `/users/me` | Bearer customer | Yes |
| `DELETE` | `/users/me` | Bearer customer | Yes (wallet must be `0.00`) |

### Request OTP

```json
{ "phone": "03001234567", "role": "customer" }
```

Roles: `customer` | `rider` | `merchant` | `admin`.

```json
{ "data": { "dev_otp": "123456" } }
```

`dev_otp` is **omitted** outside development. Limits: 5/hour/phone, 20/hour/IP.

### Verify OTP → session

```json
{ "phone": "03001234567", "role": "customer", "otp": "123456" }
```

```json
{
  "data": {
    "access_token": "...",
    "refresh_token": "...",
    "expires_in": 3600,
    "account_id": "<uuid>",
    "role": "customer",
    "status": "active"
  }
}
```

- First **customer** verify creates the user with `phone_verified: true`.
- **merchant / admin / rider** must already exist or verify returns unauthorized.

### Refresh / logout

```json
{ "refresh_token": "..." }
```

Refresh → new session. Logout → `204`.

### Google (defer for demo)

```json
{ "firebase_id_token": "..." }
```

Customer only. New Google users have `phone_verified: false` until phone link — then `POST /orders` returns `409 phone_required`.

### Phone link (only needed after Google, or if you ever create unverified customers)

1. `POST /auth/phone/link` `{ "phone": "0300..." }` → may include `dev_otp`
2. `POST /auth/phone/link/verify` `{ "otp": "..." }` → `204`

### Profile

`GET /users/me` → `{ id, name, email, phone, phone_verified, wallet_balance }`  
`PATCH /users/me` `{ "name": "..." }`  
`DELETE /users/me` → soft delete

---

## 3. Dio interceptor pattern

1. Attach `Authorization: Bearer <access_token>` when present.
2. On `401`, call `/auth/refresh` once with stored refresh token; retry original request.
3. If refresh fails, clear session and route to login.
4. Parse errors as `{ "error": { "code", "message" } }`.

Map at least: `invalid_input`, `unauthorized`, `rate_limited`, `unavailable`, `phone_required` (orders), `conflict`.

---

## 4. Demo flavor behavior

| Production | Demo flavor |
|---|---|
| User types OTP from SMS | Auto-read `dev_otp` from request response (or show it in a debug banner) |
| Google button enabled | Hide or disable Google |
| Phone link after Google | Skip — phone OTP login already verifies |

Do **not** ship a fake local JWT. Always hit the real development API so quote/place/geo share the same account.

---

## 5. Acceptance checks

- [ ] Request + verify stores tokens; subsequent `GET /users/me` returns 200.
- [ ] Refresh rotates access token without logging out.
- [ ] Logout clears storage; next API call gets 401 then login.
- [ ] Demo build never requires SMS or Firebase.
- [ ] Merchant login with `03000000001` works after seed.
- [ ] Widget tests use a fake `AuthRepository` (no network).
