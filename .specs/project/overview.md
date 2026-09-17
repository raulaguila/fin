# Overview

Serviço de identidade e autenticação em Go. API prevista em `/v1` com Bearer JWT. Swagger em `api/` (`paths` vazio). Inclui módulo exemplo `products` e outbox de e-mail.

## Estado

O modelo de identidade está no Postgres (schema `usr`); `sys.outbox` e `app.product` nas migrations. O runtime Go é bootstrap: config, logger e Postgres. Não há servidor HTTP, use cases, domínio nem worker de e-mail além do `main`.

Previsto na config, ainda não ligado: Fiber (prefork, timeouts), locales (`config.Locales`), cache Valkey/Redis (obrigatório na v1), SMTP com KEK AES-256. Fiber ainda não está em `go.mod`. RBAC: ADR-004.

## Binários

- `cmd/backend`: API. Hoje: `config.MustLoad`, logger (`service: backend`), `infra.New`. Grava outbox nos fluxos de e-mail (quando implementado).
- `cmd/mailer`: worker de e-mail. Consome `sys.outbox`; hoje só bootstrap; logger `service: mailer`.
- `cmd/admin`: CLI; `main` vazio.

Compose: `authenticity_backend`, `authenticity_mailer`, `authenticity_postgres`. Redis/Valkey comentados em `deploy/docker/services.compose.yml` (ligar na v1).
