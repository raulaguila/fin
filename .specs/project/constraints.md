# Constraints

Derivado de `config/validate.go`, CHECKs SQL e comentários de tabela.

## Ambiente protegido

`SYS_ENVIRONMENT` em `production`, `prod`, `staging` ou `stage`:

- `CORS_ORIGINS` não pode ser vazio nem `*`
- `AUTH_MANUAL_TOKEN_DELIVERY` não pode estar ligado
- SMTP habilitado exige `SMTP_HOST`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` reais (sem placeholder `example.com`)

## Auth e cache

- `AUTH_MAX_FAILED_LOGIN_ATTEMPTS` e `AUTH_ACCOUNT_LOCK_DURATION` > 0
- `CACHE_PROVIDER`: `redis` ou `valkey`

## Segredos

- Senha: só Argon2id; hash NULL até activate ou bootstrap admin
- Refresh `jti` e tokens SMTP: SHA-256; plaintext nunca gravado
- Regenerar `SMTP_MASTER_KEY` invalida senhas SMTP armazenadas; rotacionar só via re-wrap

## Produto

- Só `usr_user_status.is_login_allowed` autoriza login (hoje: `active`)
- HTTP não ecoa `failure_kind`
- `users:write` no catálogo, reservado; mutação admin de perfil fora da v1
- Sentinel `*` do admin não é linha de `usr_permission`
