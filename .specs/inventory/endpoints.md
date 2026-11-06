# Endpoints

Fonte: `Routes` de cada handler em `internal/adapter/inbound/http/rest/`, montados em `server.go`. Limites por IP e minuto: global `API_RATE_LIMIT_GLOBAL`, auth `API_RATE_LIMIT_AUTH`, senha `API_RATE_LIMIT_PASSWORD`.

| Método | Rota | Acesso | Limite |
|--------|------|--------|--------|
| GET | `/health` | público | nenhum |
| GET | `/v1/health` | público | global |
| GET | `/swagger/*` | público, só fora de produção | nenhum |
| POST | `/v1/auth/register` | público | auth |
| POST | `/v1/auth/activate` | público | auth |
| POST | `/v1/auth/login` | público | auth |
| POST | `/v1/auth/refresh` | refresh token | global |
| POST | `/v1/auth/logout` | Bearer | global |
| POST | `/v1/auth/password-reset/request` | público | senha |
| POST | `/v1/auth/password-reset/confirm` | público | senha |
| GET | `/v1/auth/me` | Bearer | global |
| GET | `/v1/auth/sessions` | Bearer | global |
| DELETE | `/v1/auth/sessions/others` | Bearer | global |
| DELETE | `/v1/auth/sessions/:id` | Bearer, só sessão própria | global |
| GET | `/v1/user/preferences` | Bearer | global |
| PUT | `/v1/user/preferences` | Bearer | global |
| GET | `/v1/user/notifications` | Bearer | global |
| GET | `/v1/user/notifications/count` | Bearer | global |
| PUT | `/v1/user/notifications/:id/read` | Bearer, só notificação própria | global |
| GET | `/v1/permissions/resources` | `permissions:read` | global |
| GET | `/v1/roles` | `roles:read`; não lista `is_superuser` nem grant `*` | global |
| POST | `/v1/roles` | `roles:create` | global |
| PATCH | `/v1/roles/:id` | `roles:update`; não `is_superuser`; padrão não desativa | global |
| DELETE | `/v1/roles/:id` | `roles:delete`; sem membros; não `is_superuser` nem padrão | global |
| PUT | `/v1/roles/:id/permissions` | `roles:update`; não `is_superuser`; mapa completo em uma transação | global |
| GET | `/v1/roles/:id/members` | `roles:read` | global |
| PUT | `/v1/roles/:id/members/:user_id` | `roles:update`; não `is_superuser` | global |
| DELETE | `/v1/roles/:id/members/:user_id` | `roles:update`; não `is_superuser` | global |
| GET | `/v1/products` | `products:read` | global |
| GET | `/v1/products/list` | `products:read` | global |
| GET | `/v1/products/:id` | `products:read` | global |
| POST | `/v1/products` | `products:create` | global |
| PATCH | `/v1/products/:id` | `products:update` | global |
| DELETE | `/v1/products/:id` | `products:delete` | global |

`/v1/auth/refresh` responde `409` ao perdedor de dois refresh simultâneos: o cliente lê o token do vencedor e repete.

Login, refresh e `/v1/auth/me` devolvem o mesmo objeto `user`, com `is_superuser`, `permissions` e `preferences`. Listas paginadas: `{items, pagination{page,limit,total_items,total_pages}}`.
