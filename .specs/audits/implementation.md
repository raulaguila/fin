# Audit: implementation

| Área | Código | Banco / config |
|------|--------|----------------|
| Bootstrap | `MustLoad` + Postgres | ok |
| HTTP | nenhum listen | rate limit, CORS, prefork só em env |
| JWT | parse de PEM RSA | emissão/refresh inexistentes |
| Argon2id | não usado no Go | CHECK na credencial |
| Sessão / refresh reuse | — | tabelas e CHECKs |
| Tokens SMTP | — | `usr_token` |
| Mailer | bootstrap só | SMTP env + aescrypt |
| Cache | `enableCache` ignorado | Valkey/Redis comentados no compose |
| Admin CLI | `main` vazio | — |
| Locales | embed declarado | pasta precisa existir para o build |
| Docker `BASE` | default `bin` no Makefile | arquivos são `built.Dockerfile` e `source.Dockerfile` |
| Testes | `pkg/aescrypt` | resto sem testes |

`HealthChecker` existe e não é chamado no bootstrap.
