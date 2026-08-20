# Commands

Executar na raiz do repositório. Quase todos os alvos Docker/migrate exigem `config/.env` (`make init` se faltar).

## Setup

```sh
make check-system
make install-tools
make init
make tidy
```

`init` gera `config/.env` via `config/env.sh` e não sobrescreve se o arquivo já existir.

## Dev

```sh
make compose-services    # Postgres (e cache se o profile estiver ativo)
make migrate-up
make dev-backend         # Air: air.backend.toml
make dev-mailer          # Air: air.mailer.toml
```

## Qualidade

```sh
make format
make lint
make audit               # mod verify, vet, govulncheck, lint
make test                # exclui ./cmd
make coverage
```

## Build

```sh
make swag
make build-backend
make build-mailer
make build-admin
make build
make zip                 # bin, Makefile, config/env.sh, version.txt, locales, deploy
```

## Migrations

```sh
make migrate-create name=descricao
make migrate-up
make migrate-down
make migrate-reset
make migrate-force version=N
make migrate-version
```

Dependem de Postgres em `localhost:${POSTGRES_PORT}` (default do env.sh: 5444).

## Compose

```sh
make compose-up
make compose-build       # backend + mailer + postgres
make compose-services    # só serviços (postgres)
make compose-down
make compose-log         # DOCKER_SERVICE=backend por padrão
make compose-exec
make compose-remove      # apaga volumes; pede confirmação
```
