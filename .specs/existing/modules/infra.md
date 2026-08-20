# infra

`internal/infra.Resources` expõe `PostgresDB`. `New` loga host/database e conecta via `postgres.Connect`. `Shutdown` fecha o `sql.DB`.

Não inicializa cache, SMTP, Fiber nem health server. O parâmetro `enableCache` não tem efeito.
