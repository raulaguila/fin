# postgres adapter

Outbound de persistência, ainda sem repositórios de domínio.

- `Connect` / `MustConnect`: GORM, `PrepareStmt`, pool 50/50, lifetime 1h, logger Silent.
- `HealthChecker.Ping`: `sql.DB.PingContext`.
- `shared.WrapError`: `ErrRecordNotFound` → `NOT_FOUND`; unique → `ALREADY_EXISTS`; FK → `RESOURCE_IN_USE`; check → `VALIDATION_FAILED`; resto → `DATABASE_ERROR`.
