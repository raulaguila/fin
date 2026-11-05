# Gaps

Restante. Runtime: `ARCHITECTURE.md`. Surfaces: `.keel/surfaces/`.

1. API roles / `usr_resource` / permissions (mutations).
2. Observability (correlation id / metrics / traces).
3. Rate limit em memória do processo: uma instância só, sem prefork.
4. Teto de e-mails por hora: cooldown de 60 s ainda permite invalidar o link a cada minuto.
5. Validação pendente: SMTP real com TLS, proxy reverso real, carga, CI no GitHub.

Não copiar multi-tenant SMTP nem dashboard do hexago.
