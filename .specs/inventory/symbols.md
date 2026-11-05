# Symbols

Símbolos exportados usados como superfície estável. Internos de `cmd` omitidos.

## config

- structs: `LogConfig`, `APIConfig`, `AuthConfig`, `PostgresConfig`, `CacheConfig`, `SMTPConfig`, `Environment`
- `Load()`, `MustLoad()`

## internal/infra

- `Resources` (`PostgresDB *gorm.DB`)
- `New(cfg, log, enableCache)`
- `(*Resources).Shutdown`

## postgres

- `Config`, `Connect`
- `HealthChecker`, `NewHealthChecker`, `(*HealthChecker).Ping`
- `shared.WrapError(err, resource)`

## pkg/apperror

- `Code`, constantes (`UNAUTHORIZED`, `INVALID_CREDENTIALS`, `ACCOUNT_*`, `NOT_FOUND`, …)
- `Error`, `New`, `IsCode`, `IsNotFound`

## pkg/aescrypt

- `ParseKey`, `Encrypt`, `Decrypt`

## pkg/logger

- `Config`, `Logger`, `New`, `ParseLevel`
- `Debug` / `Info` / `Warn` / `Error` / `Shutdown` / `With` / `Zerolog`

## pkg/smtpsecurity

- `Mode`, `ModeNone`, `ModeTLS`, `ModeStartTLS`, `Parse`, `Valid`, `UsesTLS`

## pkg/pointer

- `Of[T]`
