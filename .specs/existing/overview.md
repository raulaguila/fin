# Existing overview

O que o código faz hoje.

1. `config.MustLoad` + logger + `infra.New` (Postgres + Valkey/Redis).
2. `cmd/backend`: Fiber — health, `/v1/auth/*`, `/v1/me*`, `/v1/products` CRUD + Bearer/Require.
3. Auth: register → outbox → activate → login → refresh → logout; password reset + outbox; authz cache invalidate no activate.
4. `cmd/mailer`: poll `sys.outbox` (SKIP LOCKED), SMTP send, backoff / terminal fail.
5. Products: full CRUD on `app.product` with RBAC bits.

Repos: user, credential, session, token, attempt, role, outbox (enqueue+claim), rbac, product.

Testes unitários: aescrypt, password, domain, authorize, middleware, product validation.
