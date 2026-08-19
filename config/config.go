package config

import (
	"crypto/rand"
	"crypto/rsa"
	"embed"
	"encoding/base64"
	"fmt"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/godeh/dotenvgo"
	"github.com/golang-jwt/jwt/v5"
	"github.com/raulaguila/authenticity/pkg/smtpsecurity"
)

//go:embed locales/*
var Locales embed.FS

type LogConfig struct {
	Format string `env:"LOG_FORMAT" default:"text"`
	Level  string `env:"LOG_LEVEL" default:"info"`
}

type APIConfig struct {
	Port          int    `env:"API_PORT" default:"9999"`
	EnableLogger  bool   `env:"API_LOGGER" default:"1"`
	EnableSwagger bool   `env:"API_SWAGGO" default:"0"`
	EnablePrefork bool   `env:"API_PREFORK" default:"1"`
	CorsOrigins   string `env:"CORS_ORIGINS" default:"*"`

	GlobalRateLimit   int `env:"API_RATE_LIMIT_GLOBAL" default:"100"`
	AuthRateLimit     int `env:"API_RATE_LIMIT_AUTH" default:"10"`
	PasswordRateLimit int `env:"API_RATE_LIMIT_PASSWORD" default:"3"`

	// Timeouts applied to the Fiber/fasthttp server. Guards against slowloris
	// and leaked goroutines on stuck syscalls.
	ReadTimeout     time.Duration `env:"API_READ_TIMEOUT" default:"30s"`
	WriteTimeout    time.Duration `env:"API_WRITE_TIMEOUT" default:"30s"`
	IdleTimeout     time.Duration `env:"API_IDLE_TIMEOUT" default:"120s"`
	ShutdownTimeout time.Duration `env:"API_SHUTDOWN_TIMEOUT" default:"30s"`
}

type AuthConfig struct {
	AccessPrivateKey             *rsa.PrivateKey `env:"ACCESS_TOKEN" default:"new"`
	AccessExpiration             time.Duration   `env:"ACCESS_TOKEN_EXPIRE" default:"15m"`
	RefreshPrivateKey            *rsa.PrivateKey `env:"REFRESH_TOKEN" default:"new"`
	RefreshExpiration            time.Duration   `env:"REFRESH_TOKEN_EXPIRE" default:"60m"`
	RegistrationTokenTTL         time.Duration   `env:"REGISTRATION_TOKEN_TTL" default:"24h"`
	PasswordResetTokenTTL        time.Duration   `env:"PASSWORD_RESET_TOKEN_TTL" default:"2h"`
	MaxFailedLoginAttempts       int             `env:"AUTH_MAX_FAILED_LOGIN_ATTEMPTS" default:"5"`
	AccountLockDuration          time.Duration   `env:"AUTH_ACCOUNT_LOCK_DURATION" default:"15m"`
	AllowManualTokenDelivery     bool            `env:"AUTH_MANUAL_TOKEN_DELIVERY" default:"0"`
	LoginAttemptEmailWindow      time.Duration   `env:"AUTH_LOGIN_ATTEMPT_EMAIL_WINDOW" default:"15m"`
	LoginAttemptEmailMaxFailures int             `env:"AUTH_LOGIN_ATTEMPT_EMAIL_MAX" default:"5"`
	LoginAttemptIPWindow         time.Duration   `env:"AUTH_LOGIN_ATTEMPT_IP_WINDOW" default:"1m"`
	LoginAttemptIPMaxFailures    int             `env:"AUTH_LOGIN_ATTEMPT_IP_MAX" default:"20"`
}

type PostgresConfig struct {
	Host     string `env:"POSTGRES_HOST" default:"postgres"`
	Port     int    `env:"POSTGRES_PORT" default:"5432"`
	User     string `env:"POSTGRES_USER" default:"root"`
	Password string `env:"POSTGRES_PASS" default:"root"`
	Base     string `env:"POSTGRES_BASE" default:"authenticity"`
	URL      string `env:"POSTGRES_URL" default:"host=${POSTGRES_HOST} user=${POSTGRES_USER} password=${POSTGRES_PASS} dbname=${POSTGRES_BASE} port=${POSTGRES_PORT} sslmode=disable TimeZone=${TZ}"`
}

type CacheConfig struct {
	Backend      string        `env:"CACHE_PROVIDER" default:"valkey"`
	Host         string        `env:"CACHE_HOST" default:"cache"`
	Port         int           `env:"CACHE_PORT" default:"6379"`
	User         string        `env:"CACHE_USER" default:"default"`
	Password     string        `env:"CACHE_PASS" default:""`
	DB           int           `env:"CACHE_DB" default:"0"`
	TTL          time.Duration `env:"CACHE_TTL" default:"10m"`
	DashboardTTL time.Duration `env:"DASHBOARD_CACHE_TTL" default:"30s"`
}

type SMTPConfig struct {
	// MasterKey (SMTP_MASTER_KEY) is the KEK that wraps every per-platform DEK
	// stored in sys_platform_smtp.encrypted_dek. 64 hex chars = AES-256.
	// Regenerating the KEK makes every stored SMTP password unreadable — rotate
	// only via a re-wrap job (see docs/project/smtp-multitenancy.md phase 7).
	MasterKey string `env:"SMTP_MASTER_KEY" default:""`

	// CredentialKey is the deprecated alias of MasterKey kept for one release.
	// If set and MasterKey is empty, CredentialKey is used. Emits a deprecation
	// warning at boot. Remove in the release following the SMTP multi-tenancy
	// rollout.
	CredentialKey string `env:"SMTP_CREDENTIAL_KEY" default:""`

	// The fields below are retained until Phase 2/3 of the SMTP multi-tenancy
	// plan relocate SMTP configuration entirely into sys_platform_smtp. After
	// that, they are stripped from the struct and env.sh. See
	// docs/project/smtp-multitenancy.md.
	Enabled  bool              `env:"SMTP_ENABLED" default:"false"`
	Host     string            `env:"SMTP_HOST" default:""`
	Port     int               `env:"SMTP_PORT" default:"587"`
	User     string            `env:"SMTP_USER" default:""`
	Password string            `env:"SMTP_PASS" default:""`
	From     string            `env:"SMTP_FROM" default:""`
	ReplyTo  string            `env:"SMTP_REPLY_TO" default:""`
	Security smtpsecurity.Mode `env:"SMTP_SECURITY" default:"none"`
}

type Environment struct {
	Timezone    *time.Location `env:"TZ" default:"America/Manaus"`
	Version     string         `env:"SYS_VERSION" default:"1.0.0"`
	Environment string         `env:"SYS_ENVIRONMENT" default:"production"`

	Log      LogConfig
	API      APIConfig
	Auth     AuthConfig
	Postgres PostgresConfig
	Cache    CacheConfig
	SMTP     SMTPConfig
}

func init() {
	dotenvgo.RegisterParser(smtpsecurity.Parse)

	dotenvgo.RegisterParser(func(s string) (string, error) {
		return strings.TrimSpace(s), nil
	})

	dotenvgo.RegisterParser(func(s string) (*rsa.PrivateKey, error) {
		if s == "new" {
			return rsa.GenerateKey(rand.Reader, 2048)
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
		t, err := jwt.ParseRSAPrivateKeyFromPEM(decoded)
		if err != nil {
			return nil, err
		}
		return t, nil
	})
}

func loadDotEnv() error {
	paths := []string{path.Join("config", ".env")}

	_, currentFile, _, ok := runtime.Caller(0)
	if ok {
		paths = append(paths, path.Join(path.Dir(currentFile), "..", "config", ".env"))
	}

	var lastErr error
	for _, envPath := range paths {
		if err := dotenvgo.LoadDotEnv(envPath, true); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed to load environment file: %w", lastErr)
}

func Load() (*Environment, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}

	env := &Environment{}
	if err := dotenvgo.Load(env); err != nil {
		return nil, fmt.Errorf("failed to parse environments: %w", err)
	}

	time.Local = env.Timezone

	if err := validateEnvironment(env); err != nil {
		return nil, err
	}

	return env, nil
}

func MustLoad() *Environment {
	cfg, err := Load()
	if err != nil {
		panic(err)
	}
	return cfg
}
