# Gaps

Ordem sugerida para fechar o esqueleto. Detalhe em `tasks/backlog.md` e `features/identity/`.

1. Ligar Fiber em `cmd/backend` (dependência + listen + shutdown) sem domínio ainda, ou já com health.
2. Criar `internal/core` (domain, ports, usecase) para identidade alinhado ao schema `usr`.
3. Repositórios Postgres para user, credential, session, token, login_attempt, RBAC.
4. Fluxos: register → token activation → activate (senha + papel default) → login → refresh com rotação → logout.
5. Password reset via `usr_token`.
6. Mailer consumindo outbox ou fila (ainda não há tabela de outbox neste repo).
7. Cache real (`infra.New` / compose).
8. `cmd/admin` bootstrap do primeiro admin.
9. Corrigir default Docker `BASE=bin` vs `built.Dockerfile`.
10. Swagger preenchido a partir dos handlers, não o contrário.

Não copiar catálogo/dashboard/multi-tenant SMTP do hexago sem decisão explícita.
