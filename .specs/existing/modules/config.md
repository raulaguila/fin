# config

`config.Load` / `MustLoad` tentam `config/.env` e o path relativo ao arquivo fonte.

Structs: log, API (porta, swagger, prefork, CORS, rate limits, timeouts Fiber), Auth (RSA access/refresh, TTLs, lockout, janelas de tentativa), Postgres, Cache, SMTP (KEK + alias `CredentialKey` + host/user/pass).

`Locales` faz embed de `config/locales/*`. Timezone default `America/Manaus`.

Validação em `validate.go` (ver `project/constraints.md`).
