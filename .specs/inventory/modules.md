# Modules

| Módulo | Papel |
|--------|--------|
| `cmd/backend` | Processo da API; bootstrap |
| `cmd/mailer` | Processo do worker de e-mail; bootstrap |
| `cmd/admin` | CLI; `main` vazio |
| `config` | Env, JWT RSA, embed de locales, validação |
| `internal/infra` | `Resources` + Postgres |
| `internal/adapter/outbound/persistence/postgres` | GORM connect, health, wrap de erros PG |
| `pkg/apperror` | Erros de aplicação agnósticos de protocolo |
| `pkg/aescrypt` | AES-256-GCM, wrap KEK/DEK, bundle por passphrase |
| `pkg/logger` | Zerolog |
| `pkg/smtpsecurity` | `none` / `tls` / `starttls` |
| `pkg/pointer` | `Deref` |
| `api` | Swagger gerado |

Ausentes (pretendidos): `internal/core`, `internal/adapter/inbound`, `internal/composition`, `internal/app`.
