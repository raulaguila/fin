# Symbols

Símbolos exportados usados como superfície estável. Internos de `cmd` omitidos.

## config

- `Locales` (`embed.FS`)
- structs: `LogConfig`, `APIConfig`, `AuthConfig`, `PostgresConfig`, `CacheConfig`, `SMTPConfig`, `Environment`
- `Load()`, `MustLoad()`

## internal/infra

- `Resources` (`PostgresDB *gorm.DB`)
- `New(cfg, log, enableCache)`
- `(*Resources).Shutdown`

## postgres

- `Config`, `Connect`, `MustConnect`
- `HealthChecker`, `NewHealthChecker`, `(*HealthChecker).Ping`
- `shared.WrapError(err, resource)`

## pkg/apperror

- `Code`, constantes (`UNAUTHORIZED`, `INVALID_CREDENTIALS`, `ACCOUNT_*`, `NOT_FOUND`, …)
- `Error`, `New`, `IsCode`, `IsNotFound`, `IsValidationError`, `GetCode`

## pkg/aescrypt

- `NewDEK`, `ParseKey`, `Encrypt`, `Decrypt`, `WrapKey`, `UnwrapKey`
- `PassphraseBundle`, `SealWithPassphrase`, `OpenWithPassphrase`, `MarshalBundle`, `UnmarshalBundle`, `DeriveKey`

## pkg/logger

- `Config`, `Logger`, `New`, `ParseLevel`
- `Debug` / `Info` / `Warn` / `Error` / `Shutdown` / `With` / `Zerolog`

## pkg/smtpsecurity

- `Mode`, `ModeNone`, `ModeTLS`, `ModeStartTLS`, `Parse`, `Valid`, `UsesTLS`

## pkg/pointer

- `Deref[T]`
