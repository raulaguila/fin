# Glossary

- **activate**: passo (ainda sem HTTP) que prova a caixa, grava senha Argon2id e associa o papel default.
- **KEK / DEK**: chave mestra (`SMTP_MASTER_KEY`) que envolve chaves de dados usadas para senhas SMTP em rest.
- **`failure_kind`**: motivo interno de login falho em `usr_login_attempt`; não vai na resposta HTTP.
- **`is_login_allowed`**: coluna do lookup de status; fonte da verdade do login.
- **`jti`**: id do refresh JWT; no banco só o SHA-256.
- **reuse**: apresentar `previous_refresh_token_hash` com sessão viva; revoga com `token_reuse`.
- **sid**: id da sessão no access JWT; unidade de logout.
- **sentinela `*`**: permissão do papel `admin`; a view não expande; a aplicação aplica.
- **schema `usr`**: identidade (users, credenciais, sessões, tokens, tentativas, RBAC).
