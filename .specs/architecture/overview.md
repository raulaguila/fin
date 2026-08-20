# Architecture overview

Hexagonal. O core (inexistente) não importa adapters, Fiber, GORM, Swagger nem SQL concreto. Core define contratos; adapters traduzem o mundo externo.

## Hoje

```text
cmd/{backend,mailer,admin}
config
internal/infra
internal/adapter/outbound/persistence/postgres
pkg/{logger,apperror,aescrypt,smtpsecurity,pointer}
```

`infra.New` conecta Postgres. Sem composition root, ports, inbound ou HTTP listen.

## Quando HTTP e domínio entrarem

Ver `layers.md` e `dependency-rules.md`. Fiber e JWT já estão previstos na config; Fiber ainda não está em `go.mod`.
