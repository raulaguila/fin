# Identity — acceptance

Critérios derivados do schema. Falham hoje porque não há aplicação.

- Não autentica `pending`, `expired`, `locked`, `disabled`.
- Hash de senha nunca NULL em conta `active` (exceto janela de bootstrap admin).
- Refresh plaintext não aparece em log nem no banco.
- Reuse de refresh revoga a sessão; novo refresh não vale.
- Dois tokens `activation` unused para o mesmo user são impossíveis (índice único parcial).
- `failure_kind` não aparece no JSON de erro.
- Papel `user` é o único default; activate associa esse papel.
- `admin` tem `*`; a view pode devolver `*`; autorização trata como todas as permissões do catálogo.
- DELETE de role com usuários vinculados falha.
- Quinta tentativa inválida no mesmo token SMTP marca `used_at`.
