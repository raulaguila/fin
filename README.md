# Authenticity

Identity service in Go: registration, login, sessions, role-based permissions and account data, with a products CRUD as an example of a protected resource. `cmd/backend` serves HTTP, `cmd/mailer` sends email from the outbox, and `cmd/admin` is the operator CLI. See [ARCHITECTURE.md](ARCHITECTURE.md) for how it fits together and [PRODUCT.md](PRODUCT.md) for its constraints.

## Run with Docker

```bash
make init            # creates config/.env with fresh secrets
make compose-build   # builds and starts postgres, backend and mailer
make migrate-up      # applies the schema and configures both database logins
make admin-bootstrap # asks name, email and password of the first admin
```

The API answers on `API_PORT` (9978 in the generated file). Swagger UI is at `/swagger/index.html` outside protected environments.

For an existing installation, add `POSTGRES_MAILER_USER=authenticity_mailer` and a new `POSTGRES_MAILER_PASS` (for example, generate one with `openssl rand -hex 24`) to `config/.env`. Keep the existing signing keys and SMTP master key. Stop the mailer, run `make migrate-up` (schema through 000007 and database logins), and rebuild/restart the application containers together. Role passwords come from `config/.env`, never from versioned SQL. Running `make migrate-up` again also updates both passwords when no schema migration is pending. Missing passwords stop the command before migration; a migration failure skips login configuration. The mailer no longer falls back to the backend/owner credential. Migration 000007 builds indexes and may briefly block product writes; schedule it accordingly.

`make migrate-down` reverses one migration. Before rolling back the outbox claim migration, stop both applications and deploy the matching older binaries; the new mailer requires `claim_id`. Role rollback revokes permissions but preserves the cluster-level login, and index rollback preserves the shared pg_trgm extension.

## Develop

```bash
make dev-backend
```

```bash
make dev-mailer
```

Tests against Postgres need `TEST_POSTGRES_URL` pointing at an empty database; without it they skip:

```bash
TEST_POSTGRES_URL='postgres://postgres:test@localhost:5432/authenticity_test?sslmode=disable' go test -race ./...
```

Other targets: `make help`. After changing a handler annotation run `make swagger`.

## Configuration

`config/env.sh` writes every option to `config/.env`, each with a comment. Any `SYS_ENVIRONMENT` other than development, dev, local or test gets the production rules, enforced at boot.

## Diagnostics and pagination

JSON logging is the default; use `LOG_FORMAT=text` for an interactive console. Internal diagnostics listen on `DIAGNOSTICS_ADDRESS` (default `127.0.0.1:9091`); `/metrics` is JSON, not Prometheus exposition. Inspect it from inside a container, for example `docker exec authenticity_backend wget -qO- http://127.0.0.1:9091/metrics`. Do not publish this port without access controls. `make dev-mailer` defaults its diagnostics to port 9092 so it can run beside the backend on the same host. When launching binaries directly, choose another diagnostics port for one process. An empty address disables diagnostics; Compose healthchecks expect the default address.

For large product lists, request `/v1/products?cursor=start&limit=20`, then pass the returned `pagination.next_cursor` as `cursor`. This mode omits totals and supports `created_at desc` only. Existing `page`, `limit` and sorting behavior remains available.

The integration suite is in `tests/integration`. Regression tests cover login/reset ordering, concurrent email limits, absolute session lifetime, stale outbox claims, numeric price ordering, cursor ties, restricted mailer grants and HTTP ownership/authorization. Use a new empty database on each test run. The CI and Makefile both pin govulncheck v1.9.0; refresh local tooling with `make install-tools`.
