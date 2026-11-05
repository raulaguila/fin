# Modules

| Módulo | Papel |
|--------|--------|
| `cmd/backend` | Processo da API; bootstrap |
| `cmd/mailer` | Worker de e-mail; bootstrap |
| `cmd/admin` | CLI de operador: `promote`, `disable` |
| `config` | Env, JWT RSA, validação |
| `internal/composition` | Liga adapters aos use cases |
| `internal/core/domain` | Entidades e constantes |
| `internal/core/port` | Portas `input` e `output` |
| `internal/core/usecase` | `auth`, `admin`, `authorize`, `health`, `mailer`, `product` |
| `internal/infra` | `Resources`: Postgres e cache |
| `internal/adapter/inbound/http/rest` | Fiber v3: rotas, handlers, middleware, mapper de erros |
| `internal/adapter/outbound/persistence/postgres` | Repositórios GORM, `Store`, health, wrap de erros PG |
| `internal/adapter/outbound/cache` | Valkey/Redis |
| `internal/adapter/outbound/smtp` | Envio de e-mail |
| `pkg/apperror` | Erros de aplicação agnósticos de protocolo |
| `pkg/aescrypt` | AES-256-GCM |
| `pkg/password` | Argon2id com teto de execuções simultâneas |
| `pkg/tokens` | JWT RS256 |
| `pkg/tokenhash` | Tokens aleatórios e hash SHA-256 |
| `pkg/logger` | Zerolog |
| `pkg/smtpsecurity` | `none` / `tls` / `starttls` |
| `pkg/pointer` | `Of` |
