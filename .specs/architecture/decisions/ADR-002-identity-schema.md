# ADR-002 Schema usr

Status: aceito. Implementação: SQL aplicada nas migrations; Go ainda não usa as tabelas.

Identidade vive no schema `usr` (não `sys`/`entry`). Status é lookup com `is_login_allowed`. Credencial 1:1. Sessão guarda refresh rotacionado. Tokens SMTP separados da sessão. Login attempts append-only sem FK. RBAC: ver **ADR-004** (`(role_id, resource)` × `actions INT4` CRUD, catálogo `usr_resource`, `is_superuser` + sentinela `*`).

Fonte: `deploy/db/migrations` e `existing/infrastructure/schema.md`.
