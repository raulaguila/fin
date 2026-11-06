# Architecture

<!-- keel:architecture-schema 1 -->

## Overview

**Confirmed.** Authenticity runs as two long-lived processes plus Postgres: `cmd/backend` serves Fiber HTTP (health, auth, sessions, preferences, notifications, products CRUD); `cmd/mailer` claims `sys.outbox` rows under a 10-minute lease (`SKIP LOCKED`), opens the sealed token and sends SMTP. Composition wires use cases to Postgres. Product constraints live in `PRODUCT.md`; surface briefs in `.keel/surfaces/`.

## Boundaries

**Confirmed dependency direction (as coded):**

```text
cmd → composition / infra / adapter/inbound / config / pkg
infra → adapter/outbound/persistence/postgres (opens the connection)
composition → core/usecase + adapter/outbound
adapter/inbound → port/input (+ pkg/apperror)
adapter/outbound → port/output + domain
core/usecase → domain + ports + pkg/{apperror,password,tokenhash,pointer,validate,aescrypt,clientip,logger}
core/usecase/admin → core/usecase/auth (same identity and password rules, no second copy)
core/domain → stdlib only
```

| Boundary | Owns writes | Notes |
|----------|-------------|--------|
| `internal/core` | — | Domain + ports + use cases; no Fiber/GORM/config |
| `adapter/inbound/http/rest` | — | DTOs, route map, `apperror`→HTTP |
| `adapter/outbound/persistence/postgres` | `usr.*`, `sys.outbox` | GORM/`Exec`; `Store` = TxManager + UnitOfWork |
| `cmd/mailer` | outbox claim → SMTP | Poll 2s, batch 10; terminal fail after 8 attempts or smtp_disabled |

**Sacred (product + ADR):** JWT must not carry permission claims; authz loads live from the DB on each check. Plaintext passwords / refresh JTIs / email tokens must not be stored.

**Gap (confirmed):** Authorizer is applied on `/v1/products`; own-account routes only need Bearer. Roles are managed at runtime under `/v1/roles`. Protected roles (`is_superuser` or a `*` grant) are read-only there: full access changes only through `cmd/admin`, and the API never grants `*`. Holding `roles:update` lets someone change every other role, including their own.

## Data

**Confirmed stores:**

| Store | Role | Consistency notes |
|-------|------|-------------------|
| PostgreSQL schemas `usr`, `sys`, `app` | Source of truth | Single DB; migrations 000001–000004 |

**`usr` ownership (backend auth and account use cases):** users, credentials, sessions, email tokens, login attempts, role assignment, preferences, notifications; RBAC reader uses `vw_user_effective_permissions` + `is_superuser`.

**`sys.outbox`:** backend INSERT on register / password-reset; mailer claims `pending` → `processing` (lease in `available_at`; a crashed claim returns) → `sent`/`failed`. Tokens stay AES-GCM sealed with `SMTP_MASTER_KEY` and leave the row when done. `sys.fn_cleanup()` (daily via pg_cron) drops history older than 30 days.

**`app.product`:** owned by backend product use case (CRUD).

**Database roles:** migrations run as the owner (`POSTGRES_USER`). Backend, mailer and `cmd/admin` connect as `authenticity_app` (`POSTGRES_APP_USER`), which reads and writes rows in `usr`, `sys` and `app` and cannot change the schema. Migration 000009 creates it without login; `make db-app-role` sets its password.

**Tenancy:** single identity database; no org/tenant tables. **Inferred** as single-tenant until PRODUCT users say otherwise.

**Consistency (descriptive — confirm):** auth multi-step writes use `WithinTx` (register, activate, reset confirm). Login writes are atomic single statements instead of one transaction: `RecordFailure` counts in SQL, `RecordSuccess` only applies if the verified hash is still current, so a login in flight can never undo a reset.

## Contracts

**HTTP (confirmed):** JSON; base path `/v1` for API; health also at `/health`.

| Method | Path |
|--------|------|
| GET | `/health`, `/v1/health` |
| POST | `/v1/auth/register`, `/activate`, `/login`, `/refresh`, `/logout` |
| GET | `/v1/auth/me` (Bearer) |
| GET | `/v1/auth/sessions` (Bearer) |
| DELETE | `/v1/auth/sessions/others`, `/v1/auth/sessions/:id` (Bearer) |
| GET/PUT | `/v1/user/preferences` (Bearer) |
| GET | `/v1/user/notifications`, `/v1/user/notifications/count` (Bearer) |
| PUT | `/v1/user/notifications/:id/read` (Bearer) |
| GET | `/v1/permissions/resources` (`permissions:read`) |
| GET/POST | `/v1/roles` (`roles:read`, `roles:create`) |
| DELETE | `/v1/roles/:id` (`roles:delete`) |
| PUT | `/v1/roles/:id/permissions/:resource` (`roles:update`) |
| GET | `/v1/roles/:id/members` (`roles:read`) |
| PUT/DELETE | `/v1/roles/:id/members/:user_id` (`roles:update`) |
| GET/POST | `/v1/products` |
| GET | `/v1/products/list` |
| GET/PATCH/DELETE | `/v1/products/:id` |

**Auth tokens:** RS256; access claims `sub`, `sid`, `jti`, `iat`, `exp`; refresh same with `jti` hashed at rest. Defaults: access 15m, refresh 60m.

**Errors (confirmed):** `pkg/apperror` → JSON `{code,message,field?,details?}` via `rest/mapper`; errors raised by Fiber itself (404, 405, 413) use the same shape. Login failures are always `INVALID_CREDENTIALS`: pending, expired, disabled, locked and unknown accounts answer identically, in the same time. Register answers `201` for known emails too; internal `failure_kind` not echoed.

**User object:** login, refresh and `/v1/auth/me` return the same `user`: identity, `is_superuser`, `permissions` (resource → actions) and `preferences`, loaded live. They are response data for the interface to adapt; the server still decides each request from the database, and the JWT still carries none. If the profile fails to load, login and refresh answer the tokens without `user`, since a refresh has already spent the old token, and the request log records `Request degraded` with the cause.

**Session list:** `last_seen_at` is the time of the last login or refresh, not of the last request: it is not written per request.

**Own-account routes:** the user id comes from the verified token, never from the request, and every query filters by it. Another user's session or notification answers `404`.

**Pagination / idempotency:** `GET /v1/products` and `GET /v1/user/notifications` return `{items, pagination{page,limit,total_items,total_pages}}`; limit defaults to 20, up to 100. Products also take `search`, `sort` (allowlist) and `order`. `GET /v1/products/list` returns enabled products as id and name, up to 1000. Register is idempotent on email: an unactivated account gets a fresh token, any other answers as new and sends nothing. Outbox rows are append-only inserts (no idempotency key column today).

**Contracts (authz on wire):** Bearer middleware calls `input.Authenticator`, which validates the access JWT + live session; `Require(resource, action)` calls Authorizer (fail closed).

## Control flow

**Sync (request path, confirmed):**

1. **Register** — tx: user `pending` + empty credential + activation token hash + outbox enqueue; optional plaintext token in response if `AUTH_MANUAL_TOKEN_DELIVERY`.
2. **Activate** — tx: validate token → Argon2id hash → `active` → default role → mark token used.
3. **Login** — rate-limit checks on attempts → status/lockout → verify password → atomic credential update → create session + issue JWT pair → record attempt.
4. **Refresh** — parse refresh → load session by `sid` → reuse detection / compare-and-swap rotate → new pair. Each refresh extends the session, never past `AUTH_SESSION_MAX_LIFETIME` (30 days) from login. A lock does not stop refresh. The token rotated in the last 10 s answers `409`, without revoking: the client reads the winner's token and retries.
5. **Logout** — Bearer resolves the session → revoke it (`user_logout`).
6. **Password reset** — request enqueues outbox; confirm hashes password, revokes all sessions, marks token used.

**Notifications:** written by the auth use case for password changed (same transaction as the reset), account locked, refresh token reuse and login from a client never seen on the account (best effort: a lost notice does not fail the login). Clients poll. First login notifies nothing; after it, a client with no session left on the account counts as new. Text is English; there is no per-language template.

**Async (confirmed):** mailer claims outbox with `SKIP LOCKED`, sends SMTP (or terminal `smtp_disabled`), backoff on retryable failure.

**Health:** Postgres ping (`ok` / `down`), reused for 1 s. One request probes at a time; the others answer the last report, `down` until the first probe ends.

## Failure domains

**Confirmed what fails together:**

| Domain | Behavior |
|--------|----------|
| Postgres down | Backend/mailer boot fails; health `postgres=fail` |
| Fiber timeouts | `API_READ/WRITE/IDLE_TIMEOUT` (30s/30s/120s); shutdown `API_SHUTDOWN_TIMEOUT` 30s |
| Login abuse | DB-backed attempt counters per client+account (5), client (`/64` on IPv6) and account (50) → `TOO_MANY_REQUESTS`; lockout at 50 answers `INVALID_CREDENTIALS`. One client cannot lock the owner out. Each attempt is reserved under an advisory lock before hashing, so parallel requests cannot outrun a limit |
| Memory | Body limit 16 KiB; at most 4 Argon2 hashes (256 MiB live); `GOMEMLIMIT` and container limits in compose; Argon2 runs outside transactions and drops out when the request times out |
| Email flood | `AUTH_EMAIL_TOKEN_COOLDOWN` (60 s) and `AUTH_EMAIL_TOKEN_HOURLY_MAX` (5) per account and purpose; over the cap the live link survives |
| Refresh reuse | Session revoked `token_reuse` |

**HTTP limits:** `API_RATE_LIMIT_*` per client IP (IPv6 by `/64`) per minute, in process memory: one instance. Counters reset on restart; add `limit_req` in the proxy before scaling out. Login and email limits live in Postgres and hold across instances.

**Not implemented (confirmed):** separate DLQ table (terminal fails stay on outbox as `failed`).

**Inferred:** no circuit breaker library; outbound HTTP none on request path today.

## Observability

**Confirmed:** zerolog via `pkg/logger`; base fields `version`, `service` (`backend`|`mailer`), `pid`. Boot logs host/port/env/errors.

**Confirmed:** `RequestLog` records every 5xx with its cause, degraded answers, and every request when `API_LOGGER` is on. The auth use case logs a warning when a best-effort step fails (attempt settlement, notification, token attempt count).

**Absent (confirmed):** request/correlation IDs; metrics/traces; auth use-case structured audit beyond `usr_login_attempt` rows.

Do not log refresh tokens, passwords, or email token plaintext (payload in outbox is sensitive — treat as secret storage until mailed).

## Delivery

**Confirmed:**

- Schema: `golang-migrate` on `deploy/db/migrations`, applied by hand with `make migrate-up`, then `make db-app-role`; no container runs them
- Runtime: Docker Compose (`deploy/docker/compose.yml` + `services.compose.yml`: postgres, backend, mailer; app containers run as `nobody`). The mailer gets throwaway JWT keys (`ACCESS_TOKEN=new`, `SYS_SERVICE=mailer`), so a breach there cannot forge tokens
- Dev: Air (`make dev-backend` / `dev-mailer`); binaries `make build-*`
- Config: `config/.env` from `config/env.sh` / `make init`

**CI:** vet, tests with `-race` and Postgres, lint, `govulncheck`, and a check that `make swagger` changes nothing.

**Rollback:** SQL `*.down.sql` per migration; no feature-flag system in code. **Inferred:** expand/contract discipline is human/process, not automated.

**`cmd/admin`:** operator CLI in the backend image. `make admin-bootstrap`, or `./bin/admin` with no argument on the host, asks name, email and password and creates the first admin, active, with no email involved; it does nothing once any admin exists, and refuses an email already in use. The password is never an argument: hidden on a terminal, one line per value on a pipe. `make admin-promote` asks the email and gives that active user the `admin` role; `make admin-disable` asks the email, disables that user and revokes their sessions.

**Behind a reverse proxy:** set `API_TRUSTED_PROXIES` and keep `API_PROXY_HEADER` on a header the proxy overwrites (`X-Real-IP`), or use `X-Forwarded-For`, read from the right past the trusted proxies.

**External database:** protected environments refuse a non-local Postgres host unless `POSTGRES_URL` uses `sslmode=verify-full`.

## Cost hotspots

**Suspected (no measurements — see PRODUCT.md Unknown cost posture):**

- Argon2id verify on every login (`time=3`, `memory=64MiB`, 4 threads in `pkg/password`)
- Each permission check → one query, no cache
- Login attempt inserts + rate-limit counts
- Login, refresh and `/v1/auth/me` each run user + permissions + preferences queries
- `search` is a leading-wildcard `ILIKE`, and lists use `OFFSET` plus a count: scans grow with the table

## ADR index

| ADR | Topic |
|-----|--------|
| [ADR-001](.specs/architecture/decisions/ADR-001-hexagonal.md) | Hexagonal + Fiber |
| [ADR-002](.specs/architecture/decisions/ADR-002-identity-schema.md) | Schema `usr` |
| [ADR-003](.specs/architecture/decisions/ADR-003-secrets-at-rest.md) | Argon2id / SHA-256 / SMTP KEK |
| [ADR-004](.specs/architecture/decisions/ADR-004-rbac.md) | RBAC B+C, CRUD, live authz |

## API docs

Swagger UI at `/swagger/index.html`, outside protected environments only. Handlers carry `swag` annotations and `make swagger` regenerates `internal/adapter/inbound/http/rest/docs`. Each handler package mounts its own routes (`Routes`); every operation has one `@Tags` tag, by subject; `x-tagGroups` in `rest/swagger.go` groups the tags, rendered by the plugin in `rest/swagger.js` as collapsible sections, closed by default. `swagger_test.go` fails when a route and the docs disagree.

## Known gaps vs intent

1. **Roles/permissions mutation API** — catalog + grants via HTTP still missing.
2. **Observability thin** — no correlation id / metrics / traces.
