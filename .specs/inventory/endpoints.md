# Endpoints

Fonte: `internal/adapter/inbound/http/rest/server.go`. Limites por IP e minuto: global `API_RATE_LIMIT_GLOBAL`, auth `API_RATE_LIMIT_AUTH`, senha `API_RATE_LIMIT_PASSWORD`.

| Método | Rota | Acesso | Limite |
|--------|------|--------|--------|
| GET | `/health` | público | nenhum |
| GET | `/v1/health` | público | global |
| POST | `/v1/auth/register` | público | auth |
| POST | `/v1/auth/activate` | público | auth |
| POST | `/v1/auth/login` | público | auth |
| POST | `/v1/auth/refresh` | refresh token | global |
| POST | `/v1/auth/logout` | Bearer | global |
| POST | `/v1/auth/password-reset/request` | público | senha |
| POST | `/v1/auth/password-reset/confirm` | público | senha |
| GET | `/v1/me` | Bearer | global |
| GET | `/v1/me/permissions` | `permissions:read` | global |
| GET | `/v1/products` | `products:read` | global |
| GET | `/v1/products/:id` | `products:read` | global |
| POST | `/v1/products` | `products:create` | global |
| PATCH | `/v1/products/:id` | `products:update` | global |
| DELETE | `/v1/products/:id` | `products:delete` | global |

`/v1/auth/refresh` responde `409` ao perdedor de dois refresh simultâneos: o cliente lê o token do vencedor e repete.
