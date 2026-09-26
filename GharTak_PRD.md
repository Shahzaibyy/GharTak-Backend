# GharTak — Product Requirements Document (PRD)
**Tagline:** *Har cheez, ghar tak.*
**Version:** 1.0 | **Date:** September 2026 | **Market:** Attock District, Punjab, Pakistan

---

## 1. Executive Summary

GharTak is a hyperlocal on-demand delivery super-app built exclusively for Attock District — covering Attock City, Hazro, Fateh Jang, Jand, Pindi Gheb, and Hasan Abdal. Instead of competing head-on with national players (foodpanda, Cheetay, Bykea) in saturated metro markets, GharTak dominates an underserved regional market first: food delivery, mart/grocery delivery, and a general courier/errand service (documents, school lunches, parcels) — all inside one app, with one rider network.

Three sides: **Customers** (order anything, pay in-app), **Riders** (accept and fulfill delivery/errand tasks), **Merchants** (restaurants, marts, pharmacies list their catalog) — managed centrally through an **Admin Panel**.

## 2. Problem Statement

- Attock district's ~2 million residents have no dedicated on-demand delivery platform; national apps either don't operate here or have thin merchant coverage.
- Local shop owners have no digital storefront or delivery capability of their own.
- Informal delivery today happens through personal contacts (a shopkeeper's own boy, word-of-mouth "mazdoor") — no accountability, no tracking, no guaranteed availability, cash-only, no dispute resolution.
- No single service handles both "order food" and "send this envelope across town" — people juggle multiple informal contacts for different needs.

## 3. Market Overview

- **Geography:** All tehsils/cities of Attock District — Attock City, Hazro, Fateh Jang, Jand, Pindi Gheb, Hasan Abdal.
- **Competitive landscape:** foodpanda/Cheetay concentrate on Tier-1 cities (Islamabad, Lahore, Karachi) and have negligible or zero presence in Attock's smaller towns. Bykea offers ride-hailing/parcel in some areas but not integrated food+mart+courier under one merchant network. This leaves the district effectively open.
- **Why hyperlocal-first works:** Lower merchant acquisition cost (less competition for their attention), lower rider acquisition cost, easier to reach market density (riders profitable sooner because delivery radii are small), and a defensible moat before national players find it worth their while to enter a smaller district.

## 4. User Personas

| Persona | Description | Core need |
|---|---|---|
| **Customer** | Resident of any Attock district city, smartphone user | Order food/groceries/anything, pay easily, know when it'll arrive |
| **Rider** | Local young man with a motorbike, looking for flexible income | Steady stream of nearby delivery tasks, fast/guaranteed payout |
| **Merchant** | Restaurant, mart, pharmacy, or general store owner | Digital storefront + delivery without hiring own riders |
| **Admin/Ops** | GharTak internal team | Oversight, dispute resolution, growth metrics, fraud control |

## 5. Product Scope — Launch (all verticals together, single release)

### 5.1 Customer App (Flutter)
- Onboarding: phone number + OTP verification, optional CNIC for high-value/courier orders
- Home feed: browse by category — Restaurants, Marts/Groceries, Pharmacy, **Send a Parcel/Courier**, **Custom Errand** (school lunch, document drop, "get this from anywhere")
- Store/restaurant pages: menu/catalog, search, filters, ratings & reviews
- Cart, checkout, delivery address (map pin drop + saved addresses)
- **Custom Errand flow** (differentiator): customer describes what's needed, pickup point, drop point, optional photo — rider is matched like any other task, priced by distance + effort tier
- Live order tracking: rider's live location on Google Maps, ETA, status timeline (placed → accepted → picked up → on the way → delivered)
- Payments: JazzCash, Easypaisa, Cash on Delivery, in-app wallet (for refunds/credits)
- In-app masked calling/chat with rider (see Section 8 — Trust & Safety)
- Order history, re-order, favorites
- Ratings for both merchant and rider after delivery
- Push notifications (order status, promos)
- Support/help center + in-app complaint ticket

### 5.2 Rider App (Flutter)
- Onboarding: phone OTP, CNIC upload, bike registration/license upload, selfie verification — manual admin approval before activation
- Online/offline toggle
- Task/order notification (sound + vibration), accept/reject within a countdown window
- Navigation: turn-by-turn to pickup, then to drop-off (Google Maps SDK)
- Task types clearly flagged: Food, Mart, Parcel, Custom Errand — different pickup/drop instructions per type
- Proof of delivery: OTP entered by customer at drop-off, or photo proof for parcel/errand tasks
- In-app masked calling/chat with customer and merchant
- Earnings dashboard: per-task breakdown, daily/weekly totals, incentives/bonuses
- Wallet + cash-in-hand reconciliation (for COD orders, rider owes platform the collected cash minus their fee — settled daily/weekly)
- Rating given to customer/merchant after each task

### 5.3 Merchant Panel (Web + mobile-responsive)
- Store profile setup: name, location, category, hours, photos
- Catalog/menu management: items, prices, availability toggle, photos
- Incoming order queue: accept/reject, mark "preparing," "ready for pickup"
- Sales dashboard: daily/weekly revenue, order volume, best-selling items
- Payout tracking: commission deducted, net payout, settlement schedule

### 5.4 Admin Panel (Web)
- User management: customers, riders, merchants — view, suspend, verify
- Rider verification queue (CNIC, license, selfie match)
- Merchant onboarding approval
- Live order monitoring map — all active orders/riders across the district in real time
- Revenue dashboard: GMV, commission earned, payment method breakdown, per-city breakdown
- Dispute/complaint resolution queue
- Zone & pricing management: per-city delivery fee rules, surge multipliers, service radius
- Push notification broadcast tool (promos, announcements)
- Fraud flags dashboard (see Section 8)

## 6. Differentiators

1. **General courier/errand service** — not just food/mart. A parent can send lunch to a child's school, send documents across town, or request "go buy X from anywhere and bring it" — same rider network, same app.
2. **Total district coverage from day one** — every mart and restaurant in Attock district listed, not a curated subset.
3. **Hyperlocal trust** — smaller market means riders and merchants are more identifiable/accountable, lower fraud risk than anonymous metro-scale platforms.
4. **Single rider network across all task types** — better rider utilization (a rider isn't sitting idle waiting only for food orders).

## 7. Revenue Model

*(Industry-standard assumptions per your instruction — adjust once you have real negotiating leverage with merchants.)*

| Stream | Model | Typical rate |
|---|---|---|
| Merchant commission | % of order value, deducted at payout | 15–20% (food), 8–12% (mart/grocery — thinner margins) |
| Delivery fee (customer-paid) | Base fee + per-km rate | Rs. 50–80 base + Rs. 15–20/km |
| Custom Errand/Courier fee | Flat + distance + effort tier | Rs. 100–250 depending on complexity |
| Surge pricing | Multiplier during peak hours/bad weather | 1.2x–1.8x |
| Featured listing (merchant) | Optional paid placement in category feed | Rs. 1,000–3,000/month flat fee |
| Rider subscription (optional, later) | Reduced commission for riders paying a weekly platform fee | Phase 2 consideration |

**Take rate target (blended):** ~18–22% of GMV in year 1, improving as volume grows and fixed costs amortize.

## 8. Trust & Safety (Anti-Fraud / Anti-Off-Platform)

Direct answer to your requirement — the core mechanisms:

- **Masked communication only:** All calls/chat between customer and rider route through an in-app proxy (VoIP masking or a masked-number relay via a local telecom API) — neither party ever sees the other's real phone number. Prevents "let's skip the app next time" arrangements.
- **No payment outside app flow visible to riders:** Riders never see full order value breakdown that would tempt negotiating a private cash deal instead of using the app.
- **OTP/photo proof of delivery:** Every delivery requires a customer-side OTP or geo-tagged photo — creates an audit trail that can't be faked, and ties payout release to verified completion.
- **Repeat-pair anomaly detection:** Admin dashboard flags customer-rider pairs with an unusually high repeat-match rate outside normal proximity patterns — early signal of an off-platform arrangement being coordinated through the app.
- **Escrow-style payment holding:** Digital payments (JazzCash/Easypaisa/wallet) are held by the platform and released to merchant/rider only after delivery confirmation — protects customers from non-delivery and merchants/riders from false disputes.
- **Mandatory KYC for riders:** CNIC + selfie match + bike registration, manually verified before first task.
- **Two-way rating system:** Both sides rate each other; consistently low-rated accounts get flagged for review/suspension.
- **Account ban + blacklist sharing:** A rider or customer banned for fraud is permanently flagged (device ID + CNIC), can't simply re-register.

## 9. Success Metrics (KPIs)

- GMV (Gross Merchandise Value) per month
- Total orders/tasks completed per month, broken down by vertical (food/mart/courier)
- Active riders (daily/weekly), average tasks per rider per day
- Customer retention (repeat order rate within 30 days)
- Average delivery time (target: under 35 minutes for in-city orders)
- Merchant count live on platform, per city
- Take rate (commission + delivery fee as % of GMV)
- Fraud/dispute rate (% of orders flagged)

## 10. Roadmap

| Phase | Timeframe | Focus |
|---|---|---|
| Phase 1 — MVP Build | Months 1–3 | Core apps (customer, rider, merchant, admin), all verticals, launch in Attock City + Hasan Abdal (highest density) |
| Phase 2 — District-wide | Months 4–5 | Roll out to Hazro, Fateh Jang, Jand, Pindi Gheb — same app, new zones activated via Admin Panel |
| Phase 3 — Optimization | Months 6–9 | Route optimization, surge pricing tuning, merchant subscription tier, loyalty program |
| Phase 4 — Expansion | Month 10+ | Evaluate adjacent districts (Rawalpindi rural, Chakwal) using the same platform |

*Note: even though all three verticals launch together (per your instruction), rolling the geography out city-by-city inside Phase 1→2 is strongly recommended — it lets you concentrate your limited early rider pool where order density is highest instead of spreading them too thin across the whole district on day one.*

## 11. Cost Estimate — MVP to Launch

| Item | Estimated Cost | Notes |
|---|---|---|
| Apple Developer Program | $99/year (~Rs. 28,000) | Required for App Store |
| Google Play Console | $25 one-time (~Rs. 7,000) | One-time fee |
| Cloud servers (Go backend, Postgres, Redis) | ~$60–150/month (~Rs. 17,000–42,000) | Small VPS cluster (e.g. Hetzner/DigitalOcean) sufficient at MVP scale; Go's low footprint keeps this lean |
| Google Maps Platform (Maps SDK, Directions, live tracking) | ~$100–400/month at moderate volume | **Watch this closely** — per-load pricing scales with usage; $200/month free credit helps early on; Mapbox is a cheaper fallback if this grows too fast |
| JazzCash/Easypaisa merchant integration | ~1.5–2.5% per transaction | Contact each provider directly for current merchant rates |
| SMS/OTP gateway | ~Rs. 1–2 per SMS | Local providers (Telenor/Zong bulk SMS APIs) cheaper than international ones like Twilio |
| Push notifications (Firebase) | Free | FCM has no cost at this scale |
| Domain + SSL | ~$20/year | Negligible |
| **Estimated monthly run-rate (post-launch, pre-scale)** | **~Rs. 60,000–120,000/month** | Servers + Maps + SMS + misc; excludes team salaries |
| Development (if self-built by you + small team) | Time cost, not cash | Cash cost only applies if you hire additional developers |

## 12. Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Rider supply too thin to guarantee delivery times | Launch in 1–2 highest-density cities first, not all six at once |
| Google Maps costs spiral with scale | Set usage alerts early; evaluate Mapbox switch if costs exceed budget |
| Off-platform deals despite safeguards | Layered defense (masking + anomaly detection + rating system) — no single point of failure |
| Merchant reluctant to pay commission | Start with a lower promotional commission for first 100 merchants, raise gradually |
| Cash-on-delivery reconciliation fraud (rider under-reports cash collected) | Mandatory daily cash settlement + OTP-linked order value confirmation |

---
*Next: System Requirements Specification (SRS) — detailed functional and non-functional requirements per module.*
