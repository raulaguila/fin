package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/raulaguila/authenticity/pkg/aescrypt"
)

const minSecretLength = 16

func validateEnvironment(env *Environment) error {
	if env == nil {
		return fmt.Errorf("environment is required")
	}

	if _, err := aescrypt.ParseKey(env.SMTP.MasterKey); err != nil {
		return fmt.Errorf("SMTP_MASTER_KEY: %w", err)
	}

	if isProtectedEnvironment(env.Environment) {
		if origins := strings.TrimSpace(env.API.CorsOrigins); origins == "" || origins == "*" {
			return fmt.Errorf("CORS_ORIGINS cannot be empty or '*' in %s environment", env.Environment)
		}
		if env.Auth.AllowManualTokenDelivery {
			return fmt.Errorf("AUTH_MANUAL_TOKEN_DELIVERY cannot be enabled in %s environment", env.Environment)
		}
		if err := validateProtectedSecrets(env); err != nil {
			return err
		}
		if err := validateProtectedPostgres(env); err != nil {
			return err
		}
		if err := validateProtectedSMTP(env); err != nil {
			return err
		}
	}
	if err := validateAPIConfig(env.API); err != nil {
		return err
	}
	if err := validateAuthConfig(env.Auth); err != nil {
		return err
	}
	if env.Postgres.MaxConns <= 0 {
		return fmt.Errorf("POSTGRES_MAX_CONNS must be greater than 0")
	}
	if err := validateCacheBackend(env.Cache.Backend); err != nil {
		return err
	}

	return nil
}

func validateCacheBackend(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "redis", "valkey":
		return nil
	default:
		return fmt.Errorf("CACHE_PROVIDER must be one of redis or valkey")
	}
}

func validateAPIConfig(cfg APIConfig) error {
	if cfg.GlobalRateLimit <= 0 || cfg.AuthRateLimit <= 0 || cfg.PasswordRateLimit <= 0 {
		return fmt.Errorf("API_RATE_LIMIT_* must be greater than 0")
	}
	if cfg.RequestTimeout <= 0 {
		return fmt.Errorf("API_REQUEST_TIMEOUT must be greater than 0")
	}
	return nil
}

func validateAuthConfig(cfg AuthConfig) error {
	if cfg.MaxFailedLoginAttempts <= 0 {
		return fmt.Errorf("AUTH_MAX_FAILED_LOGIN_ATTEMPTS must be greater than 0")
	}
	if cfg.LoginAttemptPairMaxFailures <= 0 {
		return fmt.Errorf("AUTH_LOGIN_ATTEMPT_PAIR_MAX must be greater than 0")
	}
	if cfg.AccountLockDuration <= 0 {
		return fmt.Errorf("AUTH_ACCOUNT_LOCK_DURATION must be greater than 0")
	}
	// With one key an access token passes as a refresh token, and the reverse.
	if cfg.AccessPrivateKey != nil && cfg.RefreshPrivateKey != nil && cfg.AccessPrivateKey.Equal(cfg.RefreshPrivateKey) {
		return fmt.Errorf("ACCESS_TOKEN and REFRESH_TOKEN must be different keys")
	}
	return nil
}

// validateProtectedPostgres refuses a database reached over the network without
// a verified TLS connection. The compose network and loopback stay exempt.
func validateProtectedPostgres(env *Environment) error {
	cfg, err := pgconn.ParseConfig(env.Postgres.URL)
	if err != nil {
		return fmt.Errorf("POSTGRES_URL: %w", err)
	}
	if isLocalPostgres(cfg.Host) {
		return nil
	}
	if cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify {
		return fmt.Errorf("POSTGRES_URL must use sslmode=verify-full for host %s in %s environment", cfg.Host, env.Environment)
	}
	return nil
}

func isLocalPostgres(host string) bool {
	switch host {
	case "postgres", "localhost", "127.0.0.1", "::1":
		return true
	default:
		return strings.HasPrefix(host, "/") // unix socket
	}
}

func validateProtectedSecrets(env *Environment) error {
	// "new" generates a key per process: tokens fail across processes and restarts.
	for _, name := range []string{"ACCESS_TOKEN", "REFRESH_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(name)); value == "" || value == "new" {
			return fmt.Errorf("%s must hold a fixed key in %s environment", name, env.Environment)
		}
	}
	for name, value := range map[string]string{"POSTGRES_PASS": env.Postgres.Password, "CACHE_PASS": env.Cache.Password} {
		if len(value) < minSecretLength {
			return fmt.Errorf("%s must have at least %d characters in %s environment", name, minSecretLength, env.Environment)
		}
	}
	return nil
}

func validateProtectedSMTP(env *Environment) error {
	if !env.SMTP.Enabled {
		return fmt.Errorf("SMTP_ENABLED is required in %s environment: without it no account can be activated", env.Environment)
	}
	if !env.SMTP.Security.UsesTLS() {
		return fmt.Errorf("SMTP_SECURITY must be tls or starttls in %s environment", env.Environment)
	}

	required := map[string]string{
		"SMTP_HOST": env.SMTP.Host,
		"SMTP_USER": env.SMTP.User,
		"SMTP_PASS": env.SMTP.Password,
		"SMTP_FROM": env.SMTP.From,
	}

	for field, value := range required {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return fmt.Errorf("%s is required when SMTP_ENABLED=1 in %s environment", field, env.Environment)
		}
		if isSMTPPlaceholder(trimmed) {
			return fmt.Errorf("%s cannot use placeholder value in %s environment", field, env.Environment)
		}
	}

	return nil
}

func isSMTPPlaceholder(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "change-me", "changeme", "smtp.example.com", "example.com", "no-reply@example.com":
		return true
	default:
		return strings.Contains(normalized, "example.com")
	}
}

// isProtectedEnvironment fails closed: any name not known as local, including
// a typo of "production", gets the production rules.
func isProtectedEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "development", "dev", "local", "test":
		return false
	default:
		return true
	}
}
