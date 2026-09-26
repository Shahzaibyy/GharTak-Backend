# GharTak Backend — Auth & Onboarding Completion

Extends `GharTak_Next_Steps.md` Step 1. The phone-OTP flow in place works but is not the complete auth surface. This file specs what's missing: Google sign-in for customers, profile endpoints, account deletion, logout, and the presigned-upload endpoint that rider/merchant onboarding already depends on. Keep `go test ./...` green before moving on. `GharTak_Backend_Rules.md` still applies unchanged.

## Why these gaps matter

- No `/users/me` — the customer app has no way to know if a profile is complete, so it can't decide whether to show the "enter your name" screen.
- No account deletion — Apple App Store Review Guideline 5.1.1(v) and Google Play's account deletion policy both require an in-app deletion path for any app that supports account creation. This blocks store approval, not just a nice-to-have.
- No `/uploads/presign` — `GharTak_Backend_Rules.md` and the rider onboarding flow both assume presigned uploads for CNIC/selfie/vehicle-doc photos, but nothing issues one. Rider onboarding screens are not completable without it.
- No `/auth/google` — phone OTP alone is more friction than a delivery app in a competitive market needs for a first order. Google sign-in for customers only.

## Locked decisions (new)

| Topic | Decision |
|---|---|
| Google sign-in scope | Customer role only. Rider, merchant, admin stay phone-OTP-only — they need CNIC/KYC regardless, so a convenience login adds no value there. |
| Google sign-in provider | Firebase Authentication (Google provider), not a bare Google Sign-In token. The project already has a Firebase project for FCM — reuse it instead of standing up a second Google Cloud OAuth client. |
| Mobile SDK | `firebase_auth` + `google_sign_in` (Flutter). Client obtains a Firebase ID token after the native Google sheet completes. |
| Backend verification | Firebase Admin SDK for Go (`firebase.google.com/go/v4/auth`), `VerifyIDToken`. No manual JWKS handling. |
| Token issued to client | GharTak's own access + refresh JWT, same shape as `/auth/otp/verify`. The Firebase ID token is used once, at `/auth/google`, and never sent again — `auth` middleware keeps validating only GharTak-issued JWTs, unchanged. |
| `users.phone` | Becomes nullable. A Google-only signup has no phone yet. Existing `NOT NULL` constraint and its unique lookup index move to a partial unique index `WHERE phone_lookup IS NOT NULL`. |
| Phone verification gate | Not required at signup for Google users. Required before their first `POST /orders` — checkout blocks with a clear error code if `phone_verified = false`, app shows a one-screen phone-OTP prompt at that point, then retries checkout. Keeps signup to one tap; phone is only forced when the platform actually needs it (masked calling, delivery coordination). |
| Account linking | None, implicit or automatic. A Google account and a phone-OTP account are never merged by matching email or phone. If a customer wants to add a verified phone to a Google account, that's the checkout-gate flow above, written onto the same user row (not a merge). |
| Profile fields | `name`, sourced from the Firebase token's `name` claim when present; asked once, in-app, if absent (phone-OTP signups always ask, since OTP carries no name). |
| Account deletion | Soft delete: `users.deleted_at` set, `phone`/`email`/`name` overwritten with placeholders, wallet balance must be zero or refunded first. Order history rows are untouched (`customer_id` stays, for merchant/rider records and disputes) — only the `users` row is scrubbed. |
| Uploads | `POST /uploads/presign` returns a short-lived presigned PUT URL plus the `object_key` the caller must send back on the owning request (rider register, merchant catalog item, proof-of-delivery). One generic endpoint, not one per use case. |
| Logout | Revokes the specific refresh token immediately (not just on next reuse). Access token is short-lived already (~1hr) so no separate access-token revocation list is needed. |

## Schema changes

New migration, e.g. `migrations/000N_auth_google_and_profile.up.sql`:

```sql
ALTER TABLE users
    ALTER COLUMN phone DROP NOT NULL,
    ADD COLUMN email TEXT,
    ADD COLUMN firebase_uid TEXT,
    ADD COLUMN phone_verified BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN deleted_at TIMESTAMPTZ;

-- drop the old NOT NULL-backed unique index on phone_lookup if separate from a constraint, replace with:
CREATE UNIQUE INDEX users_firebase_uid_unique ON users (firebase_uid) WHERE firebase_uid IS NOT NULL;
CREATE UNIQUE INDEX users_phone_lookup_unique ON users (phone_lookup) WHERE phone_lookup IS NOT NULL;
```

Write the matching `.down.sql`. Existing rows: `phone_verified` backfills to `true` for every row that already went through OTP verify (they're all phone-OTP accounts today) — do this in the same migration, not a follow-up.

## New / changed endpoints

```
POST   /auth/google                    { firebase_id_token } → { access_token, refresh_token }  (customer only)
POST   /auth/logout                    { refresh_token } → 204, revokes it immediately

GET    /users/me                       → { id, name, email, phone (or null), phone_verified, wallet_balance }
PATCH  /users/me                       { name } → updated profile
DELETE /users/me                       → 204, soft-deletes per the table above

POST   /auth/phone/link                { } (authenticated) → sends OTP to attach/verify a phone on an existing Google-signed-in account
POST   /auth/phone/link/verify         { otp } → sets phone + phone_verified=true on the current user

POST   /uploads/presign                { purpose: "cnic"|"selfie"|"vehicle_doc"|"catalog_photo"|"proof_of_delivery" }
                                        → { upload_url, object_key, expires_in }

POST   /admin/merchants/{id}/approve
POST   /admin/merchants/{id}/reject
```

`POST /orders` gains one check: if the caller's `phone_verified` is `false`, return `409 {"error":{"code":"phone_required","message":"..."}}` instead of creating the order. The app catches this code specifically and opens the phone-link screen.

## Module changes

`internal/modules/auth`:
- Add a `firebase.Client` dependency (wraps `auth.Client` from the Admin SDK), injected in `cmd/api/main.go` like the existing pgx pool and Redis client — not constructed inside the handler.
- `GoogleSignIn(ctx, firebaseIDToken)`: verify token → look up `users` by `firebase_uid` → if none, look up nothing else (no email matching) → create a new row (`firebase_uid`, `email`, `name` from claims, `phone` null, `phone_verified=false`) → issue GharTak tokens. Existing `firebase_uid` → issue tokens for that row. Table-driven tests: new user, returning user, expired Firebase token, wrong audience.
- `PhoneLink`/`PhoneLinkVerify` reuse the existing OTP-in-Redis mechanism from Step 1 — same TTL, same hashing, same rate limits — just writing onto an already-authenticated user row instead of creating one.

New small module, `internal/platform/uploads`:
- Wraps the object storage client's presigned-URL generation. One function, `Presign(ctx, purpose, ownerID) (url, objectKey, error)`. `purpose` is a closed set of constants, dispatched via map — not an if-else chain — to the correct bucket path prefix and content-type/size limit per purpose (CNIC and selfie get tighter size limits than catalog photos).

`internal/modules/admin`:
- `ApproveMerchant`/`RejectMerchant`, same shape as the existing rider approve/reject, updating `merchants.verification_status`.

## Onboarding screen flow (for the Flutter side, so the endpoints line up with what the app needs)

**Customer:**
1. Splash → "Continue with Google" (primary button) and "Use phone number instead" (secondary link).
2. Google path: native Google sheet → Firebase ID token → `POST /auth/google` → if `name` came back from the token, go straight to Home; otherwise one name field, then Home.
3. Phone path: unchanged from what's built — phone → OTP → then always ask name (OTP carries no name) → Home.
4. First time the customer taps checkout: if `phone_required` comes back, show a single phone-entry-and-OTP screen (`/auth/phone/link` → `/auth/phone/link/verify`), then resume checkout automatically. Never shown earlier than this.
5. Profile screen gets a "Delete account" action wired to `DELETE /users/me`, with a confirmation step — required for store approval, not optional polish.

**Rider (unchanged in flow, now completable):** phone OTP → name/CNIC/vehicle form → for each document, call `POST /uploads/presign` with the matching `purpose`, PUT the file to the returned URL, send the returned `object_key` in the `/riders/register` body. Nothing about rider auth changes — this just unblocks the document-upload screens that had no endpoint to call.

## Implementation order

1. Migration + `users` schema change, backfill `phone_verified`.
2. `/users/me` GET/PATCH — smallest change, unblocks app-side profile screen work immediately.
3. `/uploads/presign` — unblocks rider onboarding screens, independent of Google work.
4. Firebase Admin SDK wiring + `/auth/google`.
5. `/auth/phone/link` + `/auth/phone/link/verify`, and the `phone_required` check on `POST /orders`.
6. `/auth/logout`, `DELETE /users/me`.
7. `/admin/merchants/{id}/approve|reject`.

Each step keeps `go test ./...` green and gets its own table-driven tests before moving to the next, same as the rest of `GharTak_Next_Steps.md`.
