# Manifest

Specs do authenticity. Fonte: o repositório atual. Inventário e `existing/` descrevem o que existe. Arquitetura pretendida, auditorias e features marcam o que ainda não está no Go.

Versão do sistema: `0.0.1` (`config/version.txt`). Módulo: `github.com/raulaguila/authenticity`.

## Árvore

- `project/` — objetivo, restrições, glossário
- `inventory/` — arquivos, módulos, símbolos, endpoints, dependências
- `existing/` — o que já corre: módulos Go, infra, integrações
- `architecture/` — hexagonal, camadas, regras, ADRs (incl. ADR-004 RBAC); `permission-models.md` histórico
- `audits/` — aderência, implementação, gaps
- `features/` — identidade, products (exemplo), mailer/outbox
- `tasks/` — backlog e o que já foi feito neste esqueleto

## Leitura

1. `project/overview.md`
2. `existing/overview.md`
3. `architecture/overview.md`
4. `architecture/decisions/ADR-004-rbac.md` (RBAC)
5. `audits/gaps.md` antes de implementar HTTP ou domínio
6. `features/identity/` para o modelo de conta já gravado no banco
