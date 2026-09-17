# Identity — technical

- Tabelas: ver `existing/infrastructure/schema.md`. RBAC: ADR-004.
- Config: `AuthConfig` (TTL access/refresh, registration, reset; lockout; janelas e-mail/IP; `AllowManualTokenDelivery`).
- Erros: `pkg/apperror` `INVALID_CREDENTIALS`, `ACCOUNT_PENDING|EXPIRED|LOCKED|DISABLED`, `TOKEN_EXPIRED`, `TOO_MANY_REQUESTS`.
- Rate limit previsto: `API_RATE_LIMIT_AUTH`, `API_RATE_LIMIT_PASSWORD`, mais janelas em `usr_login_attempt`.
- JWT: chaves RSA distintas access/refresh; `sid` na sessão; **sem** claims de permission.
- Authz: load effective map (view → cache); `is_superuser` ou `*`; senão `Allows(have, need)` CRUD.
- Comentário SQL: senha gravada em `POST /activate` — rota ainda não existe.
- Persistência futura: repositórios no adapter Postgres; `shared.WrapError`.
- E-mail: INSERT `sys.outbox`; mailer + SMTP config; envio ainda não implementado.
