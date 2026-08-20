# Dependencies

Go `1.26.5`. Diretas em `go.mod`:

| Pacote | Uso atual |
|--------|-----------|
| `github.com/godeh/dotenvgo` | Load de `config/.env` |
| `github.com/golang-jwt/jwt/v5` | Parse de chave RSA privada (access/refresh) |
| `github.com/jackc/pgx/v5` | Driver; códigos PG em `shared/errors.go` |
| `github.com/rs/zerolog` | Logger |
| `github.com/swaggo/swag` | Annotations / `api/docs.go` |
| `golang.org/x/crypto` | Argon2 não usado ainda no Go; PBKDF2 em `aescrypt` passphrase |
| `gorm.io/driver/postgres` | Dialector |
| `gorm.io/gorm` | Conexão |

Fiber não é dependência. JWT está no módulo mas nenhum token é emitido ainda.

Tooling via `make install-tools`: swag, gofumpt, goimports, govulncheck, golangci-lint, migrate, air.
