# Manifest

Specs do authenticity. Fonte: o repositório atual. Inventário e `existing/` descrevem o que existe. Arquitetura pretendida, auditorias e features marcam o que ainda não está no Go.

Versão do sistema: `0.0.1` (`config/version.txt`). Módulo: `github.com/raulaguila/authenticity`.

## Árvore

- `project/` — objetivo, restrições, glossário
- `inventory/` — arquivos, módulos, símbolos, endpoints, dependências
- `existing/` — o que já corre: módulos Go, infra, integrações
- `architecture/` — hexagonal pretendido, camadas, regras, ADRs
- `audits/` — aderência, implementação, gaps
- `features/` — especificações (hoje: identidade no SQL)
- `tasks/` — backlog e o que já foi feito neste esqueleto

## Leitura

1. `project/overview.md`
2. `existing/overview.md`
3. `architecture/overview.md`
4. `audits/gaps.md` antes de implementar HTTP ou domínio
5. `features/identity/` para o modelo de conta já gravado no banco
