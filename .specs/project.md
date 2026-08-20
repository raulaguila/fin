# Project

## Objetivo

Serviço de identidade e autenticação em Go (`github.com/raulaguila/authenticity`). Versão em `config/version.txt`. API prevista em `/v1` com Bearer JWT (Swagger em `api/`, `paths` vazio).

## Estado atual

O modelo de identidade está no Postgres (schema `usr`). O runtime Go ainda é bootstrap: carrega config, logger e conexão Postgres. Não há servidor HTTP, use cases, domínio nem worker de e-mail além do `main`.

Previsto (config/código, ainda não ligado): Fiber (prefork, timeouts), locales (`config.Locales`), cache Valkey/Redis, SMTP com KEK AES-256.

## Pontos de entrada

- `cmd/backend`: API HTTP. Hoje: `config.MustLoad`, logger (`service: backend`), `infra.New`.
- `cmd/mailer`: worker de e-mail. Mesmo bootstrap; logger `service: mailer`.
- `cmd/admin`: CLI; `main` vazio.

Compose: `authenticity_backend`, `authenticity_mailer`, `authenticity_postgres`. Cache Redis/Valkey está comentado em `deploy/docker/services.compose.yml`.

## Pacotes

- `config`: env, JWT RSA, validação (CORS e delivery manual bloqueados em production/staging).
- `internal/infra`: `Resources` com `PostgresDB`; `enableCache` ainda não abre cache.
- `internal/adapter/outbound/persistence/postgres`: conexão GORM e health.
- `pkg`: `logger`, `apperror`, `aescrypt`, `smtpsecurity`, `pointer`.
- `deploy/db/migrations`: golang-migrate, schema `usr`.
- `api`: Swagger gerado a partir de `cmd/backend/main.go`.
