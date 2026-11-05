# Identity — technical

- Tabelas: ver `existing/infrastructure/schema.md`. RBAC: ADR-004.
- Config: `AuthConfig` (TTL access/refresh, registration, reset; lockout; janelas e-mail/IP; `AllowManualTokenDelivery`).
- Erros de login: `INVALID_CREDENTIALS`, `TOO_MANY_REQUESTS` (pending/expired/disabled/locked respondem como credencial invalida). Outros: `TOKEN_EXPIRED`; refresh perdedor de corrida: `CONFLICT` (409).
- Rate limit: janelas em `usr_login_attempt` no use case (recusas sem teste de senha nao contam) e middleware `API_RATE_LIMIT_*` por IP/minuto, em memoria do processo.
- JWT: chaves RSA distintas access/refresh; claims `sub`/`sid`/`jti`; **sem** permissions.
- Authz: load effective map (view → cache); `is_superuser` ou `*`; senão `Allows(have, need)` CRUD.
- Rotas: `POST /v1/auth/{register,activate,login,refresh,logout,password-reset/request,password-reset/confirm}`.
- Persistência: repos Postgres; `shared.WrapError`. Pool `POSTGRES_MAX_CONNS` por processo.
- Argon2: no máximo 4 hashes simultâneos, sempre fora de transação; quem espera vaga desiste no timeout (`TIMED_OUT_ERROR`, sem contar falha de senha).
- E-mail: INSERT `sys.outbox` com token cifrado (`SMTP_MASTER_KEY`); mailer abre, envia e remove o token ao terminar.
