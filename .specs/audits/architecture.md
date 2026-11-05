# Audit: architecture

Aderência ao hexagonal declarado. Ver também `ARCHITECTURE.md` (Keel).

| Regra | Estado |
|-------|--------|
| Core sem adapters | Sim — `internal/core` não importa adapters |
| Inbound REST / Fiber | Sim — health + `/v1/auth/*` |
| Outbound Postgres | Sim — repos auth + RBAC reader + outbox enqueue |
| Cache outbound | Sim — Valkey/Redis no boot |
| Composition root | Sim — Health, Auth, Authorizer |
| `cmd` fino | Sim — bootstrap + listen/shutdown |
| `pkg` sem domínio de produto | Quase — `pkg/tokens` importa `port/output` (claims) |
| Dependências apontam para dentro | Sim no grafo principal |

Conclusão: hexagonal operacional para auth. Gaps: middleware authz, mailer consumer, products/roles API.
