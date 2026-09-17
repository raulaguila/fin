# Gaps

Ordem sugerida para fechar o esqueleto. Detalhe em `tasks/backlog.md` e `features/`.

1. Ligar Fiber em `cmd/backend` (dependência + listen + shutdown) sem domínio ainda, ou já com health.
2. Cache Valkey/Redis no compose e `infra.New` (obrigatório na v1; effective permissions).
3. Criar `internal/core` (domain, ports, usecase) para identidade alinhado ao schema `usr`.
4. Repositórios Postgres: user, credential, session, token, login_attempt, RBAC, outbox, product.
5. Port `Authorize` (superuser / `*` / `Allows` CRUD) com cache por `user_id`.
6. Fluxos: register → outbox activation → activate (senha + papel default) → login → refresh com rotação → logout.
7. Password reset via `usr_token` + outbox.
8. Mailer consumindo `sys.outbox` (`SKIP LOCKED` + SMTP).
9. API roles/resources/permissions + CRUD `products` (exemplo RBAC).
10. `cmd/admin` bootstrap do primeiro admin.
11. Corrigir default Docker `BASE=bin` vs `built.Dockerfile`.
12. Swagger preenchido a partir dos handlers, não o contrário.

Não copiar multi-tenant SMTP nem dashboard do hexago. Products é o único módulo de domínio exemplo na v1.
