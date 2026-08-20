# Conventions

Regras já no SQL, na config ou em `pkg`. Aplicar ao implementar HTTP, use cases e persistência.

## Segredos

- Plaintext nunca é gravado: senha = Argon2id; refresh `jti` e tokens SMTP = SHA-256 (`BYTEA`).
- `password_algorithm` só `argon2id` ou NULL (junto com hash NULL).
- Regenerar `SMTP_MASTER_KEY` invalida senhas SMTP armazenadas; rotacionar só via re-wrap.

## Identidade e auth

- Só `usr_user_status.is_login_allowed` autoriza login (hoje: só `active`).
- HTTP não ecoa `failure_kind`. Resposta pública: `invalid_credentials` / `account_locked` (e códigos `pkg/apperror`).
- `usr_login_attempt` registra também e-mail inexistente.
- Reuse de refresh (`previous_refresh_token_hash` apresentado com sessão viva) revoga a sessão (`token_reuse`).
- Papel default (`is_default`) associado no activate. Um default no banco.
- `*` do admin não está em `usr_permission`. A view não expande; a aplicação trata o sentinela.
- `users:write` está no catálogo e reservado; mutação admin de perfil fora da v1.

## Config

- Production/staging (`production`, `prod`, `staging`, `stage`): `CORS_ORIGINS` não pode ser vazio nem `*`; `AUTH_MANUAL_TOKEN_DELIVERY` não pode estar ligado; SMTP habilitado exige host/user/pass/from reais (sem placeholder `example.com`).
- `CACHE_PROVIDER`: `redis` ou `valkey`.
- `AUTH_MAX_FAILED_LOGIN_ATTEMPTS` e `AUTH_ACCOUNT_LOCK_DURATION` > 0.

## Erros e tempo

- Erros de domínio via `pkg/apperror` (código + mensagem). Adapters mapeiam para o protocolo.
- `updated_at` é trigger. Relógios de expiração, lockout e TTL de token são da aplicação.

## Migrations

- Ficam em `deploy/db/migrations`, sequenciais, golang-migrate.
- Não misturar fundação, tabelas de identidade e seed no mesmo arquivo novo.
- Não renomear histórico aplicado sem plano. Localmente, reset (`make migrate-reset`) é aceitável se o volume puder ser apagado.
