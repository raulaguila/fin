# Identity — requirements

Modelo já no SQL (`usr`). Aplicação Go ainda não implementa.

- Conta nasce `pending`, sem senha, até prova de caixa (activate).
- Só `active` autentica (`is_login_allowed`).
- Ativação vencida sem uso → `expired`, ainda sem senha.
- Lockout temporário → `locked` + `locked_until`; disable operacional → `disabled`.
- Senha Argon2id; lockout por tentativas falhas.
- Sessão é a unidade de logout; refresh rotaciona e detecta reuse.
- Tokens de e-mail: `activation` e `password_reset`, um ativo por propósito.
- Toda tentativa de login é gravada, inclusive e-mail desconhecido.
- RBAC: catálogo + papéis; `user` default sem permissões; `admin` com `*`.
- `users:write` fora da v1.
