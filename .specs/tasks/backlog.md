# Backlog

Do `audits/gaps.md`. Sem datas.

- [ ] Fiber no backend: dependência, listen, timeouts, graceful shutdown
- [ ] Health HTTP usando `postgres.HealthChecker`
- [ ] Cache Valkey/Redis no `infra.New` e compose
- [ ] `internal/core` identidade (domain, ports, usecases)
- [ ] Port Authorize (superuser / `*` / CRUD) + cache por user_id
- [ ] Repos Postgres: user, credential, session, token, attempt, RBAC, outbox, product
- [ ] Register + outbox activation + activate
- [ ] Login, refresh com rotação, logout
- [ ] Password reset + outbox
- [ ] Mailer consumir `sys.outbox` e enviar SMTP
- [ ] API roles / usr_resource / permissions
- [ ] CRUD `products` (exemplo RBAC)
- [ ] `cmd/admin` criar primeiro admin
- [ ] Alinhar `BASE` Docker (`bin` vs `built`)
- [ ] Preencher Swagger a partir dos handlers
- [ ] Testes além de `aescrypt`
