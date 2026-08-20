# Specs

Contexto operacional do authenticity. Fonte: o que já existe no repositório (migrations, config, cmd, Makefile). Não descreve rotas HTTP, use cases ou core — esses ainda não existem.

## Arquivos

- `project.md`: objetivo, binários e estado atual.
- `architecture.md`: hexagonal pretendido vs o que está implementado.
- `schema.md`: schema `usr`, tabelas, RBAC e ordem das migrations.
- `conventions.md`: regras já gravadas no SQL e na config.
- `commands.md`: Make, Air, migrate e Compose.

## Leitura sugerida

1. `project.md`
2. `architecture.md`
3. `schema.md`
4. `conventions.md` antes de mudar identidade, auth ou adapters.
5. `commands.md` antes de validar ou subir o ambiente.
