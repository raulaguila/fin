package config

import (
	"fmt"
	"strings"
)

func validateEnvironment(env *Environment) error {
	if env == nil {
		return fmt.Errorf("environment is required")
	}

	if isProtectedEnvironment(env.Environment) {
		if origins := strings.TrimSpace(env.API.CorsOrigins); origins == "" || origins == "*" {
			return fmt.Errorf("CORS_ORIGINS cannot be empty or '*' in %s environment", env.Environment)
		}
		if env.Auth.AllowManualTokenDelivery {
			return fmt.Errorf("AUTH_MANUAL_TOKEN_DELIVERY cannot be enabled in %s environment", env.Environment)
		}
		if env.SMTP.Enabled {
			if err := validateProtectedSMTP(env); err != nil {
				return err
			}
		}
	}
	if err := validateAuthConfig(env.Auth); err != nil {
		return err
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

func validateAuthConfig(cfg AuthConfig) error {
	if cfg.MaxFailedLoginAttempts <= 0 {
		return fmt.Errorf("AUTH_MAX_FAILED_LOGIN_ATTEMPTS must be greater than 0")
	}
	if cfg.AccountLockDuration <= 0 {
		return fmt.Errorf("AUTH_ACCOUNT_LOCK_DURATION must be greater than 0")
	}
	return nil
}

func validateProtectedSMTP(env *Environment) error {
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

func isProtectedEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "production", "prod", "staging", "stage":
		return true
	default:
		return false
	}
}
