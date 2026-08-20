# ADR-001 Hexagonal + Fiber

Status: aceito (intenção). Implementação: parcial.

API REST em arquitetura hexagonal. HTTP previsto em Fiber (`API_PREFORK`, timeouts em `APIConfig`). Swagger `/v1` Bearer.

Core isolado de framework. Handlers só falam com input ports.

Ainda não há Fiber no `go.mod` nem listen no `cmd/backend`.
