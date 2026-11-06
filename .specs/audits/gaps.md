# Gaps

Restante. Runtime: `ARCHITECTURE.md`. Surfaces: `.keel/surfaces/`.

1. API roles / `usr_resource` / permissions (mutations).
2. Observability (correlation id / metrics / traces).
3. Rate limit HTTP em memória: uma instância, sem prefork, zera ao reiniciar. Login e e-mail seguem no Postgres.
4. `GO-2026-5932` (x/crypto): sem release com correção; código não alcança a função vulnerável.
5. Migração 000007 cria índices sem `CONCURRENTLY`: 1,2 s com 300 mil tentativas, escrita nessas tabelas espera. Aplicar em janela de baixo uso.
6. App conecta ao Postgres como superusuário.
7. CI no GitHub não verificado: repositório sem acesso pela API.

Não copiar multi-tenant SMTP nem dashboard do hexago.
