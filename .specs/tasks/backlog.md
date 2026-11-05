# Backlog

Do `audits/gaps.md`. Sem datas.

- [x] Fiber no backend: dependência, listen, timeouts, graceful shutdown
- [x] Health HTTP usando `postgres.HealthChecker`
- [x] Cache Valkey/Redis no `infra.New` e compose
- [x] Port Authorize (superuser / `*` / CRUD) + cache por user_id
- [x] `internal/core` identidade (domain, ports, usecases auth)
- [x] Repos Postgres: user, credential, session, token, attempt, RBAC, outbox
- [x] Register + outbox activation + activate
- [x] Login, refresh com rotação, logout
- [x] Password reset + outbox
- [x] Middleware authz + Bearer nas rotas de domínio
- [x] Invalidar cache authz no activate / mudança de roles
- [x] Mailer consumir `sys.outbox` e enviar SMTP
- [x] CRUD `products` (exemplo RBAC)
- [ ] API roles / usr_resource / permissions (mutations)
- [x] `cmd/admin` criar primeiro admin e desativar usuário
- [x] Alinhar `BASE` Docker (`source` / `built`)
- [x] Migrar para Fiber v3
- [x] Rate limit HTTP, log de requisição e timeout
- [x] Testes de integração em Postgres real e CI
- [ ] Contador de rate limit compartilhado (Valkey) para mais de uma instância
- [ ] Teto de e-mails por hora, por conta
- [ ] Observability (correlation id / metrics)
