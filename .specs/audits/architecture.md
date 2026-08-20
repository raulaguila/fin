# Audit: architecture

Aderência ao hexagonal declarado.

| Regra | Estado |
|-------|--------|
| Core sem adapters | N/A — core ausente |
| Inbound REST / Fiber | Ausente; config já reserva prefork/timeouts |
| Outbound Postgres | Conexão + wrap de erro; sem repos |
| Composition root | Ausente |
| `cmd` fino | Sim: load config + `infra.New` |
| `pkg` sem domínio de produto | Sim |
| Dependências apontam para dentro | Ainda não há grafo core↔adapter |

Conclusão: o desenho está escrito nas specs e no layout de pastas (`adapter/outbound`). Falta criar core e inbound sem furar as `dependency-rules.md`.
