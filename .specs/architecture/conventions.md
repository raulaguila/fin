# Conventions

Regras do SQL, da config e de `pkg`. Aplicar ao implementar HTTP e domínio.

## Segredos

- Plaintext nunca gravado: senha Argon2id; refresh `jti` e tokens SMTP SHA-256 (`BYTEA`).
- `password_algorithm` só `argon2id` ou NULL (junto com hash NULL).

## Identidade

- Só `is_login_allowed` autoriza login.
- HTTP não ecoa `failure_kind` (`invalid_credentials` / `account_locked` + `apperror`).
- `usr_login_attempt` também para e-mail inexistente.
- Reuse de refresh revoga (`token_reuse`).
- Papel `is_default` no activate; um default no banco.
- `*` não está no catálogo; a view não expande; a aplicação trata.
- `users:write` reservado, fora da v1.

## Tempo e erros

- Erros via `pkg/apperror`; adapters mapeiam protocolo.
- `updated_at` é trigger. Expiração, lockout e TTL são da aplicação.

## Migrations

- `deploy/db/migrations`, sequenciais, golang-migrate.
- Não misturar fundação, tabelas e seed no mesmo arquivo novo.
- Não renomear histórico aplicado sem plano. Local: `make migrate-reset` se o volume puder apagar.
