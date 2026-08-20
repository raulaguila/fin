# Docker

`deploy/docker/compose.yml` inclui `services.compose.yml`. Serviços `backend` e `mailer` dependem de `postgres` healthy. Dockerfiles `bin` e `source` em `deploy/docker/backend` e `deploy/docker/mailer`.

Redis e Valkey estão comentados (profiles). Alias de rede previsto: `cache`.

Makefile default `BASE=bin`, mas os Dockerfiles chamam-se `built.Dockerfile` e `source.Dockerfile` — gap em `audits/implementation.md`.
