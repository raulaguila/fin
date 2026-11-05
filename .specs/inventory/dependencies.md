# Dependencies

Go `1.26.5`. Diretas em `go.mod`:

| Pacote | Uso atual |
|--------|-----------|
| `github.com/godeh/dotenvgo` | Load de `config/.env` |
| `github.com/gofiber/fiber/v3` | HTTP |
| `github.com/golang-jwt/jwt/v5` | JWT RS256 |
| `github.com/google/uuid` | Identificadores |
| `github.com/jackc/pgx/v5` | Driver; códigos PG; validação de `POSTGRES_URL` |
| `github.com/redis/go-redis/v9` | Cache Valkey/Redis |
| `github.com/rs/zerolog` | Logger |
| `golang.org/x/crypto` | Argon2id |
| `gorm.io/driver/postgres` | Dialector |
| `gorm.io/gorm` | Repositórios |

Tooling via `make install-tools`: gofumpt, goimports, govulncheck, golangci-lint, migrate, air.
