# Schema

Fonte: `deploy/db/migrations`. Schema `usr` = identidade; `sys` = outbox; `app` = products. `public` guarda extensões e `fn_set_updated_at`. Clock de negócio na aplicação; o trigger só preenche `updated_at`. Volume local antigo: `make migrate-reset`.

## Ordem

```text
000001_foundation                # citext, pgcrypto, schema usr, fn_set_updated_at
000002_tables                    # tabelas usr, usr_resource, view, seed usr_user_status
000003_seed_rbac                 # catalogo resources; papeis user/admin; admin is_superuser + *
000004_sys_outbox_app_products   # schema sys (outbox) + app (product)
000005_outbox_lease_cleanup      # lease do mailer, scrub de tokens, sys.fn_cleanup + pg_cron
000006_cleanup_unactivated_disable # cadastro abandonado na limpeza; revogacao account_disabled
000007_performance_indexes       # indices de FK, listagem e contagem de tentativas
```

## Status (`usr_user_status`)

`is_login_allowed` é a fonte da verdade. Só `active` é true.

- `pending` — cadastro sem prova de caixa; sem senha
- `active` — ativado
- `expired` — token de ativação venceu sem uso; sem senha
- `locked` — lockout (`locked_until` na credencial)
- `disabled` — operador

## Tabelas `usr`

- `usr_user`: e-mail `CITEXT` único; `status` FK; nome 2–80 chars.
- `usr_credential`: 1:1; Argon2id; NULL até activate; `failed_login_attempts` >= 0.
- `usr_session`: logout (`sid`) + refresh (SHA-256 do `jti`); rotação via `previous_refresh_token_hash`; revogação `user_logout` | `token_reuse` | `password_changed` | `expired` com `revoked_at`. IP/UA snapshot, não device tracking.
- `usr_token`: `activation` | `password_reset`; SHA-256; um ativo por user+purpose; 5ª recusa marca `used_at`.
- `usr_login_attempt`: append-only, inclusive e-mail inexistente; sem FK de user.

## RBAC (ADR-004)

- Catálogo `usr_resource` (códigos seed: `users`, `roles`, `permissions`, `products`). Sem tabela `usr_permission`.
- Uma linha `(role_id, resource)` com `actions INT4`. Sem FK de `resource` (permite `*`); API valida contra o catálogo ou `*`.
- Bits CRUD: `create=1`, `read=2`, `update=4`, `delete=8` (4–30 livres). `CHECK (actions >= 0)`.
- `resource` é tipo do catálogo ou sentinela `*`. Não é UUID de instância.
- `usr_role`: nome `^[a-z0-9_-]{2,40}$`; no máximo um `is_default`; `is_superuser` (bypass no Go).
- `usr_user_role`: DELETE de role com users falha (`RESTRICT`).
- `vw_user_effective_permissions`: `bit_or(actions)` por user+resource nos papéis `enabled`; não expande `*`. Superuser fora da view (`EXISTS` no Go).

Seed: papel `user` (default, zero linhas, `is_superuser=false`). Papel `admin` com `is_superuser=true` e `resource='*'` / `actions=0x7FFFFFFF`. Grants granulares via API.

## Schema `sys`

- `sys.outbox`: eventos de e-mail (`purpose` activation|password_reset; `status` pending|processing|sent|failed; `payload` JSONB). Backend INSERT; mailer consome com `SKIP LOCKED`. Índice parcial em `pending`.

## Schema `app`

- `app.product`: exemplo CRUD (`name`, `price`, `enabled`) para exercitar resource `products`.
