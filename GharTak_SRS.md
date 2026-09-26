# GharTak — Software Requirements Specification (SRS)
**Version:** 1.0 | **Companion to:** GharTak PRD v1.0 | **Standard reference:** IEEE 830 (adapted)

---

## 1. Introduction

### 1.1 Purpose
Defines the functional and non-functional requirements for GharTak — a four-sided platform (Customer, Rider, Merchant, Admin) covering food, mart/grocery, and general courier/errand delivery across Attock District.

### 1.2 Scope
Covers: Customer mobile app (Flutter), Rider mobile app (Flutter), Merchant web panel, Admin web panel, and the backend API services (Go) that connect them, including third-party integrations (Google Maps, JazzCash, Easypaisa, FCM, SMS gateway).

### 1.3 Definitions
- **Task** — a generic unit of work assigned to a rider: a food order, mart order, parcel, or custom errand.
- **Zone** — a geographic area (city/tehsil) with its own pricing rules and rider pool, managed in Admin Panel.
- **GMV** — Gross Merchandise Value, total value of orders processed.
- **KYC** — Know Your Customer, identity verification for riders.

### 1.4 References
GharTak PRD v1.0.

---

## 2. Overall Description

### 2.1 Product Perspective
GharTak is a new, independent, greenfield platform — not integrated with any existing system. Four client applications (Customer, Rider — both Flutter; Merchant, Admin — both web) talk to a shared backend via REST/WebSocket APIs.

### 2.2 User Classes and Characteristics
| Class | Technical proficiency | Primary device |
|---|---|---|
| Customer | Low–medium | Android/iOS smartphone |
| Rider | Low–medium | Android smartphone (majority Android in this market — iOS rider app is lower priority) |
| Merchant | Low | Desktop/mobile browser |
| Admin/Ops | High (internal team) | Desktop browser |

### 2.3 Operating Environment
- Customer/Rider apps: Android 8+ and iOS 14+ (Flutter cross-compiles to both)
- Merchant/Admin panels: modern browsers (Chrome, Safari, Edge) — responsive web app
- Backend: Go services containerized (Docker), deployed on Linux VMs/Kubernetes-ready
- Database: PostgreSQL (primary), Redis (caching, live location, session/queue)

### 2.4 Design & Implementation Constraints
- Must operate reliably on 3G/4G networks with intermittent connectivity (rural parts of the district) — app must gracefully queue actions offline where feasible (e.g., rider marking task complete) and sync when reconnected.
- Google Maps API costs must be monitored — architecture should minimize redundant map loads/API calls (e.g., throttle live location updates to every 5–10 seconds, not continuous streaming).
- Payment integrations limited to what JazzCash/Easypaisa APIs officially support.

### 2.5 Assumptions and Dependencies
- Riders own an Android smartphone with GPS and mobile data.
- JazzCash/Easypaisa merchant API access is approved before Phase 1 launch.
- Google Maps Platform account with billing configured (Mapbox as documented fallback per PRD Section 11).

---

## 3. Functional Requirements

### 3.1 Customer App

| ID | Requirement |
|---|---|
| FR-C01 | System shall allow registration/login via phone number + OTP. |
| FR-C02 | System shall display merchants (restaurants, marts, pharmacies) filtered by the customer's current city/zone. |
| FR-C03 | System shall allow browsing a merchant's catalog, adding items to cart, and adjusting quantities. |
| FR-C04 | System shall allow customer to submit a **Custom Errand** request with free-text description, pickup location, drop location, and optional photo attachment. |
| FR-C05 | System shall calculate and display delivery fee and estimated delivery time before order confirmation. |
| FR-C06 | System shall support payment via JazzCash, Easypaisa, Cash on Delivery, and in-app wallet balance. |
| FR-C07 | System shall display live rider location on a map from the moment a rider accepts the task until delivery. |
| FR-C08 | System shall notify the customer via push notification at each order status change. |
| FR-C09 | System shall require a delivery OTP (or accept a rider-submitted photo, for non-OTP-eligible task types) to mark an order "delivered." |
| FR-C10 | System shall allow customer to rate the rider and merchant (1–5 stars + optional comment) after delivery. |
| FR-C11 | System shall allow the customer to contact the rider only via in-app masked call/chat — real phone numbers are never exposed to either party. |
| FR-C12 | System shall maintain order history and allow one-tap re-order. |
| FR-C13 | System shall allow the customer to raise a support ticket/dispute tied to a specific order. |

### 3.2 Rider App

| ID | Requirement |
|---|---|
| FR-R01 | System shall require CNIC upload, selfie capture, and bike registration/license upload during onboarding; account remains inactive until admin approval. |
| FR-R02 | System shall allow rider to toggle online/offline availability. |
| FR-R03 | System shall push new task notifications only to riders within a configurable radius of the pickup point, and only when online. |
| FR-R04 | System shall require rider to accept or reject a task within a countdown window (default 30 seconds); unaccepted tasks re-broadcast to the next nearest rider. |
| FR-R05 | System shall provide turn-by-turn navigation to pickup, then to drop-off, via Google Maps SDK. |
| FR-R06 | System shall transmit rider's live GPS location to the backend at a throttled interval (5–10 sec) while a task is active. |
| FR-R07 | System shall require OTP entry (customer-provided) or geo-tagged photo proof to mark a task "delivered." |
| FR-R08 | System shall display an earnings dashboard: per-task breakdown, daily/weekly totals, pending payout. |
| FR-R09 | System shall track cash-on-delivery amounts collected per rider and reconcile against the platform's expected remittance. |
| FR-R10 | System shall allow the rider to rate the customer/merchant after task completion. |
| FR-R11 | System shall allow rider to contact the customer only via in-app masked call/chat. |

### 3.3 Merchant Panel

| ID | Requirement |
|---|---|
| FR-M01 | System shall allow merchant registration with store details, category, hours, and location pin; account activated after admin verification. |
| FR-M02 | System shall allow merchant to create/edit/delete catalog items with price, description, photo, and availability toggle. |
| FR-M03 | System shall display incoming orders in real time with accept/reject and status update controls (preparing → ready for pickup). |
| FR-M04 | System shall display a sales dashboard: daily/weekly revenue, order count, top-selling items. |
| FR-M05 | System shall display payout history with commission deducted and net amount, per settlement cycle. |

### 3.4 Admin Panel

| ID | Requirement |
|---|---|
| FR-A01 | System shall provide a live map view of all active tasks and online riders across all zones. |
| FR-A02 | System shall allow admin to approve/reject/suspend rider, merchant, and customer accounts. |
| FR-A03 | System shall provide a revenue dashboard broken down by city, vertical (food/mart/courier), and payment method. |
| FR-A04 | System shall allow admin to configure per-zone delivery pricing rules and surge multipliers. |
| FR-A05 | System shall surface a fraud-flag queue highlighting anomalous customer-rider repeat-pairing patterns (see PRD Section 8). |
| FR-A06 | System shall provide a dispute resolution workspace linking to the relevant order, chat log, and proof-of-delivery evidence. |
| FR-A07 | System shall allow admin to broadcast push notifications to all users or a filtered segment (by city, user type). |

### 3.5 Backend / Platform-Level

| ID | Requirement |
|---|---|
| FR-B01 | System shall expose REST APIs for all client-facing actions and a WebSocket channel for live location and order-status updates. |
| FR-B02 | System shall route all in-app calls through a masking/proxy layer so no party's real phone number is exposed. |
| FR-B03 | System shall hold customer payment in escrow (platform-controlled ledger) until delivery is confirmed, then release merchant/rider payout. |
| FR-B04 | System shall log every order's full lifecycle (timestamps, status changes, location trail) for dispute resolution and analytics. |
| FR-B05 | System shall run a scheduled job to flag rider-customer pairs whose repeat-match frequency exceeds a configurable threshold, for admin review. |

---

## 4. External Interface Requirements

| Interface | Purpose | Notes |
|---|---|---|
| Google Maps Platform (Maps SDK, Directions, Geocoding) | Live tracking, navigation, address entry | Monitor usage against PRD cost estimate; Mapbox as fallback |
| JazzCash Merchant API | In-app payment | Requires merchant account approval |
| Easypaisa Merchant API | In-app payment | Requires merchant account approval |
| Firebase Cloud Messaging (FCM) | Push notifications | Free tier sufficient |
| SMS/OTP Gateway (local telecom API) | Phone verification, delivery OTP | Prefer local provider over Twilio for cost |
| Masked Calling/VoIP Proxy | Customer↔rider communication without exposing numbers | Local telecom masked-call API or a VoIP relay service |

---

## 5. Non-Functional Requirements

| Category | Requirement |
|---|---|
| **Performance** | API response time < 500ms for 95% of requests under normal load; live location updates delivered to customer app within 5 seconds of rider ping. |
| **Scalability** | Backend must handle at least 5,000 concurrent active tasks district-wide without degradation (headroom well above Phase 1 volume). |
| **Availability** | 99.5% uptime target for core ordering/payment services. |
| **Security** | All traffic over TLS; passwords/OTPs never logged; payment credentials never stored on GharTak servers (delegate to JazzCash/Easypaisa tokenized flows); role-based access control on Admin Panel. |
| **Data Privacy** | Real phone numbers, CNIC images, and payment details encrypted at rest; access restricted to authorized admin roles only. |
| **Usability** | Customer app onboarding-to-first-order in under 3 minutes for a new user. |
| **Offline resilience** | Rider app queues status updates locally and syncs on reconnect if network drops mid-task. |
| **Maintainability** | Backend services structured as independently deployable modules (orders, payments, riders, notifications) to allow isolated updates. |
| **Auditability** | Every payment, status change, and admin action logged with timestamp and actor ID. |

---

## 6. Core Data Model (entities)

- **User** (customer): id, phone, name, saved addresses, wallet balance
- **Rider**: id, phone, CNIC, verification status, vehicle info, current zone, rating, online status
- **Merchant**: id, name, category, zone, catalog items, commission rate, verification status
- **Order/Task**: id, type (food/mart/courier/errand), customer_id, merchant_id (nullable for courier), rider_id, pickup, drop, status, price breakdown, payment method, timestamps
- **Payment**: id, order_id, amount, method, status (held/released/refunded)
- **Rating**: id, order_id, rater_id, ratee_id, score, comment
- **Zone**: id, city name, pricing rules, active status
- **FraudFlag**: id, involved user/rider ids, reason, status (open/reviewed/dismissed)

---

## 7. Key Use Case Flows

**UC-1: Place a food order**
Customer browses restaurant → adds items to cart → confirms address → selects payment → order sent to merchant → merchant accepts → nearest available rider notified → rider accepts → picks up → delivers with OTP → payment released → both sides rate.

**UC-2: Custom Errand**
Customer opens "Custom Errand" → describes task + pickup/drop + photo → system estimates price → customer confirms → nearest rider notified → rider accepts → completes pickup/drop → photo proof submitted → payment released.

**UC-3: Fraud flag review**
Scheduled job detects a customer-rider pair repeat-matched 5+ times outside normal proximity pattern → flag created → appears in Admin fraud queue → admin reviews order/chat history → decides: dismiss, warn, or suspend.

**UC-4: Cash-on-delivery reconciliation**
Rider completes a COD task → app records cash amount owed to platform → amount accumulates in rider's "cash owed" balance → rider settles (bank transfer/in-person) on the scheduled cycle → admin marks reconciled.

---

## 8. Appendix — Glossary
See Section 1.3 for core terms. Additional terms will be added as the system evolves.

---
*Next: Technical Design Document — architecture diagram, service breakdown, database schema, API contract, and infrastructure layout.*
