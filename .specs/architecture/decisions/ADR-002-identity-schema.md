# ADR-002 Schema usr

Status: aceito. Implementação: SQL aplicada nas migrations; Go ainda não usa as tabelas.

Identidade vive no schema `usr` (não `sys`/`entry`). Status é lookup com `is_login_allowed`. Credencial 1:1. Sessão guarda refresh rotacionado. Tokens SMTP separados da sessão. Login attempts append-only sem FK. RBAC N:N com sentinela `*` só no admin.

Fonte: `deploy/db/migrations` e `existing/infrastructure/schema.md`.
