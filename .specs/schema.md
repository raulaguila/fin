# Schema

Fonte: `deploy/db/migrations`. Schema `usr` = identidade (users, credenciais, sessões, tokens, tentativas de login, papéis, catálogo de permissões). `public` guarda extensões e `fn_set_updated_at`. Clock de negócio continua na aplicação; o trigger só preenche `updated_at`.

## Ordem

```text
000001_foundation   # citext, pgcrypto, schema usr, fn_set_updated_at
000002_tables       # tabelas, view, seed de usr_user_status
000003_seed_rbac    # catálogo, papéis user e admin
```

## Status da conta (`usr_user_status`)

`is_login_allowed` é a fonte da verdade do login. Só `active` é true.

| code     | login | significado                                      |
|----------|-------|--------------------------------------------------|
| pending  | não   | cadastro sem prova de caixa; sem senha           |
| active   | sim   | ativado                                          |
| expired  | não   | token de ativação venceu sem uso; sem senha      |
| locked   | não   | lockout temporário (`locked_until` na credencial)|
| disabled | não   | desligado por operador                           |

## Tabelas

- `usr_user`: identidade. E-mail `CITEXT` único. `status` FK para o lookup. Nome 2–80 chars.
- `usr_credential`: 1:1 com user. Hash Argon2id; NULL até activate (ou bootstrap admin). `failed_login_attempts` >= 0; `locked_until` NULL = não bloqueado.
- `usr_session`: unidade de logout (`sid` no JWT) e refresh vigente (SHA-256 do `jti`). Rotação: `previous_refresh_token_hash`; apresentar o hash anterior com sessão viva = reuse. Revogação: `user_logout` \| `token_reuse` \| `password_changed` \| `expired`, sempre com `revoked_at`. IP/UA são snapshot de login, não device tracking.
- `usr_token`: SMTP, fora da sessão. `purpose`: `activation` \| `password_reset`. Hash SHA-256; plaintext nunca gravado. Um token ativo por user+purpose. `attempts`: na 5ª recusa após lookup, marca `used_at`.
- `usr_login_attempt`: append-only, inclusive e-mail inexistente. Sem FK para user. `failure_kind` interno (HTTP não ecoa o valor).

Os comentários SQL mencionam `POST /activate` como o passo que grava a senha. Esse handler ainda não existe.

## RBAC

- `usr_permission`: catálogo. Sentinel `*` não é linha.
- `usr_role`: nome `^[a-z0-9_-]{2,40}$`. No máximo um `is_default` (índice parcial). Associado no activate.
- `usr_role_permission`: N:N. Sem FK para o catálogo porque admin guarda `*`.
- `usr_user_role`: N:N. DELETE de role com users falha (`ON DELETE RESTRICT`).
- `vw_user_effective_permissions`: união das permissões dos papéis `enabled`. Não expande `*`; a aplicação aplica o sentinela.

Seed (`000003`): permissões `users:read`, `users:write` (reservado, fora da v1), `roles:read`, `roles:write`, `roles:assign`, `permissions:read`. Papel `user` (default, zero permissões). Papel `admin` com `*`.
