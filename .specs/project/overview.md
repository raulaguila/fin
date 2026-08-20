# Overview

Serviço de identidade e autenticação em Go. API prevista em `/v1` com Bearer JWT. Swagger em `api/` (`paths` vazio).

## Estado

O modelo de identidade está no Postgres (schema `usr`). O runtime Go é bootstrap: config, logger e Postgres. Não há servidor HTTP, use cases, domínio nem worker de e-mail além do `main`.

Previsto na config, ainda não ligado: Fiber (prefork, timeouts), locales (`config.Locales`), cache Valkey/Redis, SMTP com KEK AES-256. Fiber ainda não está em `go.mod`.

## Binários

- `cmd/backend`: API. Hoje: `config.MustLoad`, logger (`service: backend`), `infra.New`.
- `cmd/mailer`: worker de e-mail. Mesmo bootstrap; logger `service: mailer`.
- `cmd/admin`: CLI; `main` vazio.

Compose: `authenticity_backend`, `authenticity_mailer`, `authenticity_postgres`. Redis/Valkey comentados em `deploy/docker/services.compose.yml`.
