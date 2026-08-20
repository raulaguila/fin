# Dependency rules

- `internal/core` não importa `internal/adapter`, Fiber, GORM, Swag, `config` de infra além do necessário, nem `cmd`.
- Adapters inbound dependem de input ports, não de GORM.
- Adapters outbound dependem de output ports (implementam) e de models.
- `cmd` e composition importam adapters e core; o contrário não.
- `pkg` é utilitário sem regra de domínio (exceto códigos em `apperror`, que são agnósticos de protocolo).
- REST DTO: tags `json` / validate / swagger. Core DTO: sem tags de transporte.
- Novas features: use case → ports → adapters. Não gravar regra de login só no handler.
