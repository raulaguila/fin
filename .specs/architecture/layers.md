# Layers

## Pretendido

1. **Inbound** — `internal/adapter/inbound/http/rest` (Fiber). DTOs e mappers no adapter. Handlers chamam input ports via composition/`Application`, nunca repositórios.
2. **Core** — `internal/core/domain`, `port/input`, `port/output`, `usecase`. Sem tags de transporte. Use case devolve entidade; REST DTO fica no inbound.
3. **Outbound** — Postgres implementa output ports. Models e mappers no adapter. Erros via `shared.WrapError` → `apperror`.
4. **Runtime** — `internal/infra` recursos; `internal/composition` monta concretos; `cmd/*` só bootstrap.

## Atual

Só a camada 3 parcial (conexão, não repositórios) e 4 parcial (`infra` + `cmd`). Camadas 1 e 2 ausentes.
