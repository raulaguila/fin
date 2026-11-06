# Gaps

Restante. Runtime: `ARCHITECTURE.md`. Surfaces: `.keel/surfaces/`.

1. Quem tem `roles:update` pode ampliar o próprio acesso em qualquer recurso, menos `*`.
2. Observability (correlation id / metrics / traces).
3. Rate limit HTTP em memória: uma instância, sem prefork, zera ao reiniciar. Login e e-mail seguem no Postgres.
4. `GO-2026-5932` (x/crypto): sem release com correção; código não alcança a função vulnerável.
5. CI no GitHub não verificado: repositório sem acesso pela API.

Não copiar multi-tenant SMTP nem dashboard do hexago.
