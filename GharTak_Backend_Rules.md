# GharTak Backend — Engineering Rules (for AI agents / Cursor)

**How to use this file:** Drop this at repo root as `.cursorrules`, or under `.cursor/rules/backend.mdc`. Every task/PR against the Go backend must follow these rules without exception. If a rule and a task instruction conflict, flag it — don't silently violate the rule.

---

## 0. Framework Decision — locked in, don't relitigate

**Use `chi` (`github.com/go-chi/chi/v5`) as the HTTP router.**

Why chi over Gin/Echo/Fiber for this project:
- Built directly on `net/http` — zero lock-in, so any module can be peeled into a standalone service later (per the modular-monolith → microservices path) without a framework rewrite.
- Plays natively with `gorilla/websocket` / `nhooyr.io/websocket` for the live-location socket — no fasthttp incompatibility headaches (which Fiber has).
- Minimal magic: middleware is just `func(http.Handler) http.Handler` — easy for an AI agent to reason about and easy to test with plain `net/http/httptest`.
- Composable sub-routers map cleanly onto our module boundaries (`r.Mount("/orders", orders.Router())`).

Do not introduce Gin, Echo, or Fiber into this codebase. Do not mix routers.

---

## 1. Project Structure

```
/cmd/api/main.go                # composition root only — wiring, no business logic
/internal/modules
  /auth/        {handler.go, service.go, repository.go, model.go}
  /orders/
  /dispatch/
  /payments/
  /notifications/
  /admin/
/pkg/                           # only truly generic, project-agnostic helpers
/migrations/                    # SQL migrations, numbered, never edited after merge
```

Rules:
- A module's `repository.go` is the ONLY file that imports the database driver. Services never write raw SQL.
- A module's `service.go` is the ONLY place business logic lives. Handlers do not contain business logic — they parse input, call the service, map the result/error to an HTTP response. Nothing else.
- Cross-module calls go through the other module's exported interface, never by reaching into its repository or internal types directly.

---

## 2. Control Flow — this is the rule you asked to enforce strictly

**The failure mode to eliminate: AI-generated Go that's a wall of nested `if/else`, redundant `!= nil` checks on values that can't be nil, and defensive re-validation scattered through every layer. This is where most bugs hide. Follow these without exception:**

1. **Guard clauses only — never nested if/else.** Return/continue early on the failure case; the success path stays unindented at the bottom of the function.
   ```go
   // WRONG
   if user != nil {
       if user.IsActive {
           if order.Amount > 0 {
               // do the thing, 3 levels deep
           } else { return errInvalidAmount }
       } else { return errInactive }
   } else { return errNotFound }

   // RIGHT
   if user == nil { return errNotFound }
   if !user.IsActive { return errInactive }
   if order.Amount <= 0 { return errInvalidAmount }
   // do the thing, 0 levels deep
   ```

2. **Validate once, at the boundary. Trust it after that.** Input validation happens in the handler (or a single validation middleware). Once a value has passed validation and entered the service layer, do not re-check it "just in case" deeper in the call stack. If a function's Go signature says it returns `(*Order, error)`, the caller checks `err != nil` **once** and then trusts `*Order` is valid — no follow-up `if order != nil` after an already-checked error.

3. **No nil-checks on values the type system already guarantees.** If a constructor or factory function always returns a fully-populated struct (never a partial one), downstream code does not defensively check its fields for zero-values "to be safe." Fix it at construction time instead of scattering checks everywhere it's used.

4. **Branch on type/kind via a dispatch table or interface, never a growing if-else/switch chain repeated in multiple places.** Example — order type handling (food/mart/courier/errand):
   ```go
   // WRONG — repeated in every function that needs type-specific behavior
   if order.Type == "food" { ... } else if order.Type == "mart" { ... } else if order.Type == "courier" { ... }

   // RIGHT — one place, dispatch by interface
   type TaskHandler interface {
       CalculatePrice(order Order) (Money, error)
       ValidatePickupDrop(order Order) error
   }
   var handlers = map[OrderType]TaskHandler{
       Food: FoodHandler{}, Mart: MartHandler{}, Courier: CourierHandler{}, Errand: ErrandHandler{},
   }
   handlers[order.Type].CalculatePrice(order)
   ```

5. **Cyclomatic complexity ceiling: max 4 branches per function.** If a function needs a 5th `if`, extract a helper or move to the dispatch-table pattern above. This is a hard review-blocking rule, not a suggestion.

6. **Errors are values, checked once, wrapped with context — never used as flow-control via string matching.**
   ```go
   if err != nil {
       return fmt.Errorf("dispatch: finding nearest rider: %w", err)
   }
   ```
   Use `errors.Is` / `errors.As` against sentinel/typed errors — never `if err.Error() == "..."`.

---

## 3. SOLID, applied to Go (no classes, but the principles hold via interfaces)

- **Single Responsibility**: one struct/file = one reason to change. `orders.Service` doesn't calculate payment commission — it calls `payments.Service`.
- **Open/Closed**: extend via new interface implementations, not by editing existing ones. Adding a new payment gateway = a new `PaymentGateway` implementation, zero changes to existing gateway code.
  ```go
  type PaymentGateway interface {
      Charge(ctx context.Context, amount Money, ref string) (TxnID, error)
  }
  ```
- **Liskov**: any `PaymentGateway` implementation must honor the same contract (no gateway-specific hidden preconditions the caller has to know about).
- **Interface Segregation**: small, single-purpose interfaces. Don't define one giant `Repository` interface with 20 methods — split by use (`OrderReader`, `OrderWriter`) if a consumer only needs read access.
- **Dependency Inversion**: services depend on interfaces, injected via constructor. No global singletons, no `init()`-time DB connections. Everything wired explicitly in `cmd/api/main.go`.

---

## 4. DRY — with judgment

- Extract shared logic (pagination, response envelopes, ID generation, money math) into `/pkg` only once it's duplicated **3 times**, not on the first duplication (rule of three) — premature abstraction creates the wrong coupling more often than duplication does.
- Never DRY across module boundaries by importing another module's internals. If two modules need the same logic, it belongs in `/pkg`, not borrowed from one module by the other.

---

## 5. Database Best Practices (PostgreSQL, ACID, indexing)

**Transactions**
- Any multi-step write that must succeed or fail together (order creation + escrow hold + commission calc) runs inside one `pgx.Tx`. No partial-commit paths.
- Default isolation (`READ COMMITTED`) is fine for most reads. Use `SELECT ... FOR UPDATE` row locking for balance mutations (wallet, rider cash-owed) to prevent race conditions on concurrent updates — do not "check-then-write" without a lock.
- Payment webhook handlers must be idempotent — store and check the gateway's transaction reference before applying a charge/release, so a retried webhook can't double-process.

**Indexing**
- Every foreign key gets an index.
- Composite indexes match actual query patterns — e.g. `(zone_id, is_online)` on `riders` for the dispatch lookup, not two separate single-column indexes.
- Partial indexes where the query always filters a boolean/status: `CREATE INDEX ON riders (zone_id) WHERE is_online = true;`
- Run `EXPLAIN ANALYZE` on any new query before merging — no query ships without a known plan. Flag sequential scans on tables expected to grow past a few thousand rows.

**Query hygiene**
- Never `SELECT *` — name the columns you need.
- No N+1 queries — use a single JOIN or a batched `WHERE id = ANY($1)` instead of looping and querying per row.
- Always paginate (`LIMIT`/`OFFSET` or keyset pagination for large tables) — no endpoint returns an unbounded result set.
- All queries parameterized (`$1, $2...`) — never string-concatenated SQL, no exceptions, even for "trusted" internal values.
- Use a connection pool (`pgxpool`), sized to the deployment's actual core count — don't default to an arbitrarily large pool.

---

## 6. Redis Best Practices

- Use two Redis endpoints in production when possible: `REDIS_URL` for cache, OTP, sessions, geo, and pub/sub; `REDIS_QUEUE_URL` for asynq only with AOF (`appendonly yes`) and `maxmemory-policy noeviction`. Never run asynq on an `allkeys-lru` instance.
- Background work goes through `internal/platform/queue` (asynq): retries with backoff, delayed jobs, priority queues (`critical`, `default`, `low`), cron via the scheduler, unique task ids, and task timeouts. FCM and offer re-dispatch run in the worker, not in the HTTP transaction.
- Cross-module fan-out uses `internal/platform/events`. With `NATS_URL` set, order status is published on JetStream subject `ghartak.events.order.status` and the worker enqueues notification jobs. Without NATS, the direct publisher enqueues the same jobs.
- Redis is a cache/ephemeral layer, never the source of truth for money or order state — Postgres is authoritative. If Redis is flushed, no financial state should be lost.
- Every cache key has a `TTL`. No unbounded keys.
- Use the right structure for the job: `GEOADD`/`GEOSEARCH` for rider positions, `HASH` for structured objects needing partial field updates, plain `SET`/`GET` for simple cached values — not everything as a serialized JSON blob.
- Never run `KEYS *` in production code — it blocks the server. Use `SCAN` with a cursor.
- Set a `maxmemory` policy (`allkeys-lru` or similar) so Redis degrades gracefully instead of OOMing.
- Batch related commands with pipelining instead of issuing them one round-trip at a time.
- Pub/sub (used for live-location fan-out) is fire-and-forget — never route anything through it that requires guaranteed delivery. Use Redis Streams (or a real queue) for anything that must not be silently dropped.

---

## 7. Security Best Practices

- Parameterized queries always (see Section 5) — this is also the SQL-injection defense, not a separate concern.
- Every request-scoped operation (DB call, external API call) gets a `context.Context` with a sane timeout. No unbounded blocking calls anywhere.
- Secrets (DB creds, JazzCash/Easypaisa keys, JWT signing key) come from environment variables or a secret manager — never hardcoded, never committed, never logged.
- JWT: short-lived access tokens, refresh-token rotation, signature verified on every request via middleware (not per-handler).
- CNIC images and phone numbers encrypted at rest; object storage buckets private by default, signed URLs for any temporary access.
- Rate-limit all public endpoints (especially OTP request and order creation) — per-IP and per-account.
- Run `govulncheck` in CI on every PR; don't merge with known-vulnerable dependencies.
- Structured logs must never contain raw phone numbers, CNIC numbers, OTPs, or payment tokens — mask or omit them at the logging layer, not case-by-case at call sites.

---

## 8. Architecture-Level Rules

- **Layering is one-directional**: `handler → service → repository`. A repository never calls a service. A service never imports `net/http`.
- **Errors are translated once**, at the handler layer — domain errors (`ErrNotFound`, `ErrInsufficientBalance`) map to HTTP status codes in one central mapper function, not scattered `if err == ...` blocks per handler.
- **Config is a single typed struct**, loaded once at startup (`cmd/api/main.go`), passed by injection. No `os.Getenv` calls scattered through business logic.
- **Every module's public surface is an interface** — other modules and tests depend on that interface, not the concrete struct, so mocking in tests is trivial and swapping an implementation later doesn't ripple outward.

---

## 9. Testing

- Table-driven tests for anything with more than 2 input variations — this also naturally discourages the if-else-pyramid style, since table-driven cases push you toward the dispatch-table pattern in the code under test too.
- Mock at interface boundaries (`PaymentGateway`, repository interfaces) — no real network/DB calls in unit tests.
- Every bug fix ships with a regression test reproducing it first.

---

## 10. Definition of Done (checklist for every task)

- [ ] No nested if/else beyond one guard-clause level; cyclomatic complexity ≤ 4 branches per function
- [ ] No redundant nil-checks on values already guaranteed non-nil by an earlier `err != nil` check
- [ ] All new queries indexed appropriately and `EXPLAIN ANALYZE`-checked
- [ ] All new endpoints rate-limited and input-validated at the handler boundary only
- [ ] Multi-step writes wrapped in a transaction
- [ ] No secrets, PII, or tokens in logs
- [ ] Interfaces used at every module boundary; tests mock against them
- [ ] Table-driven tests added for new logic
