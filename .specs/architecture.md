# Architecture

## Regra principal

Arquitetura hexagonal. O core (ainda inexistente) não importa adapters, Fiber, GORM, Swagger nem SQL concreto. Core define contratos; adapters traduzem o mundo externo.

## O que existe

```text
cmd/{backend,mailer,admin}
config
internal/infra                          # bootstrap técnico
internal/adapter/outbound/persistence/postgres
pkg/{logger,apperror,aescrypt,smtpsecurity,pointer}
```

`infra.New` conecta Postgres e devolve `Resources`. Shutdown fecha o SQL DB. Não há composition root, input/output ports nem adapter inbound.

## Pretendido (quando o HTTP e o domínio entrarem)

- Inbound REST em `internal/adapter/inbound/http/rest` (Fiber).
- Handlers chamam input ports, nunca repositórios.
- Entidades e regras em `internal/core/domain`.
- Input ports em `internal/core/port/input`; output ports em `internal/core/port/output`.
- Use cases em `internal/core/usecase`.
- Models GORM no adapter Postgres; erros de banco mapeados para `pkg/apperror`.

## Runtime previsto na config (não ligado)

- Fiber: `API_PREFORK`, timeouts de read/write/idle/shutdown.
- Rate limits globais, de auth e de senha.
- JWT access/refresh com chaves RSA distintas.
- Cache: `CACHE_PROVIDER` `redis` ou `valkey`.
- SMTP env + `SMTP_MASTER_KEY` (KEK) e alias `SMTP_CREDENTIAL_KEY`.
- Locales: `//go:embed locales/*` em `config`.

## Erros

`pkg/apperror` é agnóstico de protocolo (HTTP, gRPC, CLI). Códigos de conta alinhados ao schema: `INVALID_CREDENTIALS`, `ACCOUNT_PENDING`, `ACCOUNT_EXPIRED`, `ACCOUNT_LOCKED`, `ACCOUNT_DISABLED`, etc.
