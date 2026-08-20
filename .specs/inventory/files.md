# Files

Layout relevante. Gerados (`api/docs.go`, `api/swagger.json`) entram porque o Makefile os regenera. `config/.env` é local e gitignored.

```text
cmd/backend/main.go
cmd/mailer/main.go
cmd/admin/main.go
config/config.go
config/validate.go
config/env.sh
config/version.txt
internal/infra/resources.go
internal/adapter/outbound/persistence/postgres/connection.go
internal/adapter/outbound/persistence/postgres/health.go
internal/adapter/outbound/persistence/postgres/shared/errors.go
pkg/apperror/error.go
pkg/aescrypt/aescrypt.go
pkg/aescrypt/passphrase.go
pkg/aescrypt/aescrypt_test.go
pkg/logger/logger.go
pkg/smtpsecurity/security.go
pkg/pointer/pointer.go
api/docs.go
api/swagger.yaml
deploy/db/migrations/000001_foundation.{up,down}.sql
deploy/db/migrations/000002_tables.{up,down}.sql
deploy/db/migrations/000003_seed_rbac.{up,down}.sql
deploy/docker/compose.yml
deploy/docker/services.compose.yml
deploy/docker/postgres/Dockerfile
deploy/docker/backend/{built,source}.Dockerfile
deploy/docker/mailer/{built,source}.Dockerfile
Makefile
air.backend.toml
air.mailer.toml
go.mod
```
