# Schema

Fonte: `deploy/db/migrations`. Schema `usr` = identidade. `public` guarda extensões e `fn_set_updated_at`. Clock de negócio na aplicação; o trigger só preenche `updated_at`.

## Ordem

```text
000001_foundation   # citext, pgcrypto, schema usr, fn_set_updated_at
000002_tables       # tabelas, view, seed de usr_user_status
000003_seed_rbac    # catálogo, papéis user e admin
```

## Status (`usr_user_status`)

`is_login_allowed` é a fonte da verdade. Só `active` é true.

- `pending` — cadastro sem prova de caixa; sem senha
- `active` — ativado
- `expired` — token de ativação venceu sem uso; sem senha
- `locked` — lockout (`locked_until` na credencial)
- `disabled` — operador

## Tabelas

- `usr_user`: e-mail `CITEXT` único; `status` FK; nome 2–80 chars.
- `usr_credential`: 1:1; Argon2id; NULL até activate; `failed_login_attempts` >= 0.
- `usr_session`: logout (`sid`) + refresh (SHA-256 do `jti`); rotação via `previous_refresh_token_hash`; revogação `user_logout` | `token_reuse` | `password_changed` | `expired` com `revoked_at`. IP/UA snapshot, não device tracking.
- `usr_token`: `activation` | `password_reset`; SHA-256; um ativo por user+purpose; 5ª recusa marca `used_at`.
- `usr_login_attempt`: append-only, inclusive e-mail inexistente; sem FK de user.

## RBAC

- Catálogo `usr_permission`; `*` não é linha.
- `usr_role`: nome `^[a-z0-9_-]{2,40}$`; no máximo um `is_default`.
- `usr_role_permission` sem FK para o catálogo (admin guarda `*`).
- `usr_user_role`: DELETE de role com users falha (`RESTRICT`).
- `vw_user_effective_permissions`: papéis `enabled`; não expande `*`.

Seed: `users:read`, `users:write` (fora da v1), `roles:read`, `roles:write`, `roles:assign`, `permissions:read`. Papel `user` (default, zero perms). Papel `admin` com `*`.
