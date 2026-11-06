# Product

<!-- keel:product-schema 1 -->

## Platform

`web-api` + `worker` (evidence: `cmd/backend` Fiber HTTP API; `cmd/mailer` outbox consumer planned/bootstrap).

## Stack

- Language / runtime: Go (`go.mod` module `github.com/raulaguila/authenticity`, Go 1.26.x)
- HTTP: Fiber v3
- Datastore: PostgreSQL 17 (schemas `usr`, `sys`, `app`; golang-migrate)
- Auth crypto: RSA JWT (access/refresh), Argon2id passwords (PHC), SHA-256 for refresh `jti` and email tokens
- SMTP: env + AES KEK/DEK helpers (`pkg/aescrypt`); mailer not consuming outbox yet
- Deploy: Docker Compose (`authenticity_backend`, `authenticity_mailer`, `authenticity_postgres`)

## Users

Unknown — confirm before designing to that constraint.

Hypothesis from purpose: applications and operators that need identity (register/login/session) and RBAC; not confirmed as named customers or first-party apps.

## Product Purpose

Provide identity and authentication for clients: account lifecycle (pending → activate → active), session-based JWT access/refresh with rotation and reuse detection, session management by the user, password reset, live RBAC (resource × CRUD), user preferences, in-app security notifications, and email delivery via Postgres outbox.

Operational success (from specs, not measured SLOs): only `active` accounts authenticate; plaintext secrets never at rest; HTTP never echoes login `failure_kind`; authz does not trust JWT permission claims.

## Positioning

**Is:** a focused authenticity/identity service with hexagonal layout, identity schema `usr`, system outbox `sys`, and a small example domain module `app.product` to exercise RBAC.

**Is not (v1):** multi-tenant SMTP catalog, product commerce platform, ABAC/instance-level authz, or a general dashboard — explicitly out of scope vs copying “hexago” extras without a decision.

## Operating Context

- Local/dev via Compose and `config/.env` (`SYS_ENVIRONMENT=development` allows `AUTH_MANUAL_TOKEN_DELIVERY`)
- Protected envs (`production`/`staging`): CORS cannot be `*`; manual token delivery forbidden; SMTP requires real settings when enabled
- Tenancy: single-tenant identity store (no org/tenant tables in migrations)
- Regions / on-call / HA topology: Unknown — confirm before designing to that constraint.

## Scale & SLOs

Unknown — confirm before designing to that constraint.

(Config exposes rate-limit knobs and lockout windows; no measured QPS, latency targets, or availability SLOs in-repo.)

## Cost Posture

Unknown — confirm before designing to that constraint.

Known expensive-by-design paths (qualitative): Argon2id verify on login; authz Postgres view on each check; mailer/SMTP when enabled.

## Capabilities and Constraints

**Must**

- Hexagonal dependency direction (core ≠ Fiber/GORM)
- Password Argon2id only; refresh/`usr_token` stored as SHA-256
- Login gated by `usr_user.status` (only `active` today)
- RBAC B+C (ADR-004): `usr_resource` catalog; actions CRUD bits; admin `is_superuser` and/or `resource='*'`; live authz via view
- Email side effects via `sys.outbox` (backend enqueue; mailer consume)

**Must not (v1)**

- Put permissions in access JWT
- Echo internal `failure_kind` on HTTP
- Multi-tenant SMTP / hexago dashboard copy without explicit decision
- Instance-level (UUID) authorization — type-level RBAC only

## Evidence on Hand

- `.specs/project/overview.md`, `constraints.md`, `glossary.md`
- `.specs/architecture/decisions/ADR-001` … `ADR-004` (hexagonal, identity schema, secrets, RBAC)
- `.specs/features/identity/*`, `mailer/outbox.md`, `products/requirements.md`
- `.specs/existing/overview.md`, `existing/infrastructure/schema.md`
- Code: `cmd/backend`, `internal/core/usecase/auth`, `internal/adapter/inbound/http/rest`, migrations `000001`–`000004`

## Product Principles

1. **Secrets stay hashed or wrapped** — never store password, refresh `jti`, or email token plaintext.
2. **Session is the logout unit** — access carries `sid`; refresh rotates and detects reuse.
3. **Authz is live and typed** — load effective permissions at request time; authorize resource types with CRUD bits (or superuser/`*`).
4. **Side effects are durable first** — email goes through outbox, then the mailer.
5. **Specs and SQL constrain the product** — CHECKs and ADRs beat ad-hoc feature creep; example `products` exists only to prove RBAC.
6. **Fail closed on account state** — non-`active` (and lockout) must not authenticate; HTTP responses stay coarse for credential failures.
