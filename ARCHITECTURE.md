# Architecture

<!-- keel:architecture-schema 1 -->

## Overview

**Confirmed.** Authenticity runs as two long-lived processes plus Postgres and Valkey/Redis: `cmd/backend` serves Fiber HTTP (health, auth, me, products CRUD); `cmd/mailer` claims `sys.outbox` rows under a 10-minute lease (`SKIP LOCKED`), opens the sealed token and sends SMTP. Composition wires use cases to Postgres/cache. Product constraints live in `PRODUCT.md`; surface briefs in `.keel/surfaces/`.

## Boundaries

**Confirmed dependency direction (as coded):**

```text
cmd → composition / infra / adapter/inbound / config / pkg
composition → core/usecase + adapter/outbound
adapter/inbound → port/input (+ pkg/apperror)
adapter/outbound → port/output + domain
core/usecase → domain + ports + pkg/{apperror,password,tokenhash,pointer}
core/domain → stdlib only
```

| Boundary | Owns writes | Notes |
|----------|-------------|--------|
| `internal/core` | — | Domain + ports + use cases; no Fiber/GORM/config |
| `adapter/inbound/http/rest` | — | DTOs, route map, `apperror`→HTTP |
| `adapter/outbound/persistence/postgres` | `usr.*`, `sys.outbox` | GORM/`Exec`; `Store` = TxManager + UnitOfWork |
| `adapter/outbound/cache` | Valkey keys | Authz effective map only today |
| `cmd/mailer` | outbox claim → SMTP | Poll 2s, batch 10; terminal fail after 8 attempts or smtp_disabled |

**Sacred (product + ADR):** JWT must not carry permission claims; authz loads live from DB/cache. Plaintext passwords / refresh JTIs / email tokens must not be stored.

**Gap (confirmed):** Authorizer is applied on protected routes (`/v1/me/permissions`, `/v1/products`, logout via Bearer). Role/permission mutation APIs still missing.

## Data

**Confirmed stores:**

| Store | Role | Consistency notes |
|-------|------|-------------------|
| PostgreSQL schemas `usr`, `sys`, `app` | Source of truth | Single DB; migrations 000001–000007 |
| Valkey/Redis (`CACHE_*`) | Authz cache | Key `authz:effective:{userID}`; TTL `CACHE_TTL` (default 10m). Boot **fails** if cache unreachable when `enableCache=true` |

**`usr` ownership (backend auth use case):** users, credentials, sessions, email tokens, login attempts, role assignment; RBAC reader uses `vw_user_effective_permissions` + `is_superuser`.

**`sys.outbox`:** backend INSERT on register / password-reset; mailer claims `pending` → `processing` (lease in `available_at`; a crashed claim returns) → `sent`/`failed`. Tokens stay AES-GCM sealed with `SMTP_MASTER_KEY` and leave the row when done. `sys.fn_cleanup()` (daily via pg_cron) drops history older than 30 days.

**`app.product`:** owned by backend product use case (CRUD).

**Tenancy:** single identity database; no org/tenant tables. **Inferred** as single-tenant until PRODUCT users say otherwise.

**Consistency (descriptive — confirm):** auth multi-step writes use `WithinTx` (register, activate, reset confirm). Login writes are atomic single statements instead of one transaction: `RecordFailure` counts in SQL, `RecordSuccess` only applies if the verified hash is still current, so a login in flight can never undo a reset. Authz cache `Invalidate` runs after activate (default role grant).

## Contracts

**HTTP (confirmed):** JSON; base path `/v1` for API; health also at `/health`.

| Method | Path |
|--------|------|
| GET | `/health`, `/v1/health` |
| POST | `/v1/auth/register`, `/activate`, `/login`, `/refresh`, `/logout` |
| GET | `/v1/me` (Bearer) |
| GET | `/v1/me/permissions` (Bearer + `permissions:read`) |
| GET/POST | `/v1/products` |
| GET/PATCH/DELETE | `/v1/products/:id` |

**Auth tokens:** RS256; access claims `sub`, `sid`, `jti`, `iat`, `exp`; refresh same with `jti` hashed at rest. Defaults: access 15m, refresh 60m.

**Errors (confirmed):** `pkg/apperror` → JSON `{code,message,field?,details?}` via `rest/mapper`. Login failures are always `INVALID_CREDENTIALS`: pending, expired, disabled, locked and unknown accounts answer identically, in the same time. Register answers `201` for known emails too; internal `failure_kind` not echoed.

**Pagination / idempotency:** `GET /v1/products` returns full list (no cursor/limit yet). Register is idempotent on email: an unactivated account gets a fresh token, any other answers as new and sends nothing. Outbox rows are append-only inserts (no idempotency key column today).

**Contracts (authz on wire):** Bearer middleware validates access JWT + live session; `Require(resource, action)` calls Authorizer (fail closed).

## Control flow

**Sync (request path, confirmed):**

1. **Register** — tx: user `pending` + empty credential + activation token hash + outbox enqueue; optional plaintext token in response if `AUTH_MANUAL_TOKEN_DELIVERY`.
2. **Activate** — tx: validate token → Argon2id hash → `active` → default role → mark token used.
3. **Login** — rate-limit checks on attempts → status/lockout → verify password → atomic credential update → create session + issue JWT pair → record attempt.
4. **Refresh** — parse refresh → load session by `sid` → reuse detection / compare-and-swap rotate → new pair. A lock does not stop refresh. The token rotated in the last 10 s answers `409`, without revoking: the client reads the winner's token and retries.
5. **Logout** — parse access → revoke session (`user_logout`).
6. **Password reset** — request enqueues outbox; confirm hashes password, revokes all sessions, marks token used.

**Async (confirmed):** mailer claims outbox with `SKIP LOCKED`, sends SMTP (or terminal `smtp_disabled`), backoff on retryable failure.

**Health:** aggregates Postgres ping + cache ping (`ok` / `degraded` / `down`).

## Failure domains

**Confirmed what fails together:**

| Domain | Behavior |
|--------|----------|
| Postgres down | Backend/mailer boot fails; health `postgres=fail` |
| Cache down | Backend boot fails (`infra.New` with cache); health would mark cache fail if process were up without it |
| Fiber timeouts | `API_READ/WRITE/IDLE_TIMEOUT` (30s/30s/120s); shutdown `API_SHUTDOWN_TIMEOUT` 30s |
| Login abuse | DB-backed attempt counters → `TOO_MANY_REQUESTS`; lockout answers `INVALID_CREDENTIALS` |
| Memory | Body limit 16 KiB; at most 4 Argon2 hashes (256 MiB live); `GOMEMLIMIT` and container limits in compose; Argon2 runs outside transactions and drops out when the request times out |
| Email flood | `AUTH_EMAIL_TOKEN_COOLDOWN` (60 s) per account and purpose |
| Refresh reuse | Session revoked `token_reuse` |

**HTTP limits:** `API_RATE_LIMIT_*` per client IP (IPv6 by `/64`) per minute, in process memory (one instance; no prefork).

**Not implemented (confirmed):** separate DLQ table (terminal fails stay on outbox as `failed`).

**Inferred:** no circuit breaker library; outbound HTTP none on request path today.

## Observability

**Confirmed:** zerolog via `pkg/logger`; base fields `version`, `service` (`backend`|`mailer`), `pid`. Boot logs host/port/env/errors.

**Absent (confirmed):** request/correlation IDs; HTTP access log middleware (`API_LOGGER` unused); metrics/traces; auth use-case structured audit beyond `usr_login_attempt` rows.

Do not log refresh tokens, passwords, or email token plaintext (payload in outbox is sensitive — treat as secret storage until mailed).

## Delivery

**Confirmed:**

- Schema: `golang-migrate` on `deploy/db/migrations`; compose service `migrate` applies it before backend and mailer start (`make migrate-*` for manual use)
- Runtime: Docker Compose (`deploy/docker/compose.yml` + `services.compose.yml`: postgres, valkey, migrate, backend, mailer; app containers run as `nobody`)
- Dev: Air (`make dev-backend` / `dev-mailer`); binaries `make build-*`
- Config: `config/.env` from `config/env.sh` / `make init`

**Rollback:** SQL `*.down.sql` per migration; no feature-flag system in code. **Inferred:** expand/contract discipline is human/process, not automated.

**`cmd/admin`:** operator CLI in the backend image. `make admin-promote email=…` gives an active user the `admin` role (first admin included); `make admin-disable email=…` disables the user and revokes their sessions.

**Behind a reverse proxy:** set `API_TRUSTED_PROXIES` and keep `API_PROXY_HEADER` on a header the proxy overwrites (`X-Real-IP`), or use `X-Forwarded-For`, read from the right past the trusted proxies.

**External database:** protected environments refuse a non-local Postgres host unless `POSTGRES_URL` uses `sslmode=verify-full`.

## Cost hotspots

**Suspected (no measurements — see PRODUCT.md Unknown cost posture):**

- Argon2id verify on every login (`time=3`, `memory=64MiB`, 4 threads in `pkg/password`)
- Authz cache miss → view + superuser query
- Login attempt inserts + rate-limit counts

## ADR index

| ADR | Topic |
|-----|--------|
| [ADR-001](.specs/architecture/decisions/ADR-001-hexagonal.md) | Hexagonal + Fiber |
| [ADR-002](.specs/architecture/decisions/ADR-002-identity-schema.md) | Schema `usr` |
| [ADR-003](.specs/architecture/decisions/ADR-003-secrets-at-rest.md) | Argon2id / SHA-256 / SMTP KEK |
| [ADR-004](.specs/architecture/decisions/ADR-004-rbac.md) | RBAC B+C, CRUD, live authz |

## Known gaps vs intent

1. **Roles/permissions mutation API** — catalog + grants via HTTP still missing.
2. **Observability thin** — no correlation id / metrics / traces.
3. **Rate limit counters in process memory** — move to Valkey before running more than one backend instance.
