# GharTak — Flutter API: Customer & rider onboarding (HTML screens)

Maps [Bhook Lagi_ onboarding v3.html](./Bhook%20Lagi_%20onboarding%20v3.html) to backend routes.  
**Index:** [GharTak_Flutter_Integration_Index.md](./GharTak_Flutter_Integration_Index.md)  
**Auth details:** [GharTak_Flutter_API_Auth.md](./GharTak_Flutter_API_Auth.md)

Welcome carousel + role picker are **client-only**. Category tiles (Khaana/Grocery/Dasti) after onboarding can soft-save preferences.

Until Meta / Resend / Hetzner are configured: use `APP_ENV=development` (`dev_otp`), local unsigned upload URLs, and hide Google if Firebase is unavailable.

---

## Customer flow

| Screen | API |
|---|---|
| Number + Send code (WhatsApp) | `POST /auth/otp/request` `{phone, role:"customer", channel:"whatsapp"}` |
| Send via SMS instead | same with `channel:"sms"` |
| Continue with Google | `POST /auth/google` `{firebase_id_token}` (503 until Firebase configured) |
| Sign up with email | `POST /auth/email/request` `{email}` → `POST /auth/email/verify` `{email,otp}` |
| Code daalo / Verify | `POST /auth/otp/verify` `{phone, role:"customer", otp}` |
| Naam aur jagah | `POST /customers/onboarding` `{name, address:{label,lat,lng,address_text}}` |
| Kya mangwana hai | `PATCH /customers/me/preferences` `{preferred_order_types:["food","mart","courier"]}` |

Address `label`: `home` | `work` | `other`.  
Email / Google users must phone-link before `POST /orders` (`phone_required`).

Demo: use `dev_otp` from request response — no WhatsApp or Resend needed.

---

## Rider flow

| Screen | API |
|---|---|
| Apply (phone + city) | `POST /riders/onboarding/apply` `{phone, zone_id, channel?}` then `POST /auth/otp/verify` `{phone, role:"rider", otp}` |
| Already a rider? Log in | `POST /auth/otp/request` + `verify` with `role:"rider"` (must exist) |
| Apni details | `PATCH /riders/me/onboarding/details` `{name,cnic,vehicle_type,vehicle_reg?,license_number?}` |
| Documents | `POST /uploads/presign` per purpose → PUT JPEG → `PUT /riders/me/onboarding/documents` |
| Submit | `POST /riders/me/onboarding/submit` |
| Check / training | `GET /riders/me/onboarding` (timeline) |
| Book orientation | `POST /riders/me/onboarding/orientation` `{preferred_slot?}` |
| Contact support | `POST /tickets` |

### Upload purposes

`cnic_front`, `cnic_back`, `driving_licence`, `selfie`  
(Aliases kept: `cnic` → front path, `vehicle_doc` → licence path.)

### Go online

`POST /riders/availability` `{is_online:true}` requires **approved** + zone + orientation `booked` or `completed`.

City UI: load `GET /zones` and send `zone_id` (not free-text city).

Legacy one-shot: `POST /riders/register` still works for tests.

---

## Flutter placement

```
features/onboarding/
  customer/   # phone/email/google → onboarding form → preferences
  rider/      # apply → details → docs → submit → timeline
```

Reuse shared auth repository for OTP/email/Google. Map envelopes and money rules from the integration index.

---

## Backend deps (later)

| Env | Used for |
|---|---|
| `MAPBOX_BACKEND_TOKEN` | Maps pricing/geo (separate guide) |
| Firebase credentials | Google button |
| Meta WhatsApp / SMS | Real OTP delivery |
| `RESEND_API_KEY` | Email OTP |
| `S3_*` Hetzner endpoint | Real document storage |

Until then, stubs log sends and development returns `dev_otp`.
