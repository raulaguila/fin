# Authenticity

Identity service in Go: registration, login, sessions, role-based permissions and account data, with a products CRUD as an example of a protected resource. `cmd/backend` serves HTTP, `cmd/mailer` sends email from the outbox, and `cmd/admin` is the operator CLI. See [ARCHITECTURE.md](ARCHITECTURE.md) for how it fits together and [PRODUCT.md](PRODUCT.md) for its constraints.

## Run with Docker

```bash
make init            # creates config/.env with fresh secrets
make compose-build   # builds and starts postgres, backend and mailer
make migrate-up      # applies the schema (needs the migrate tool: make install-tools)
make db-app-role     # gives the application its database login
make admin-bootstrap # asks name, email and password of the first admin
```

The API answers on `API_PORT` (9978 in the generated file). Swagger UI is at `/swagger/index.html` outside protected environments.

An existing `config/.env` keeps working without `POSTGRES_APP_USER` and `POSTGRES_APP_PASS`, but then the processes connect as the database owner. Add both, run `make db-app-role` and restart to use the restricted role.

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
