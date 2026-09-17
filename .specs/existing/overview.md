# Existing overview

O que o código faz hoje, sem o hexagonal completo.

1. `config.MustLoad` lê `config/.env`, valida ambiente e monta chaves RSA (ou gera se `ACCESS_TOKEN`/`REFRESH_TOKEN` = `new`).
2. Logger Zerolog com `version` e `service`.
3. `infra.New` abre GORM em Postgres. `enableCache` é ignorado.
4. Processo permanece vivo só enquanto o `main` não retorna — não há listen HTTP nem loop de mailer.

Persistência: schemas SQL `usr` (identidade), `sys` (outbox), `app` (product). Nenhuma query de user/session/token/outbox no Go.

Testes: `pkg/aescrypt` (round-trip encrypt, wrap KEK/DEK, bundle passphrase).

RBAC e outbox: ADR-004 e `features/mailer/outbox.md`. Cache ainda não ligado (`enableCache` ignorado).
