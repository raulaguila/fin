#!/bin/bash

release_version=$(cat config/version.txt | tr -d "[:space:]")
# Ed25519 keys sign the JWTs; access and refresh must differ.
access_token=$(openssl genpkey -algorithm ed25519 | base64 | tr -d \\n)
refresh_token=$(openssl genpkey -algorithm ed25519 | base64 | tr -d \\n)
# 32 bytes (AES-256) as 64-char hex. Seals email tokens in sys.outbox.
smtp_master_key=$(openssl rand -hex 32)
postgres_pass=$(openssl rand -hex 12)
postgres_app_pass=$(openssl rand -hex 12)
postgres_mailer_pass=$(openssl rand -hex 12)

echo "TZ='America/Manaus'                             # Set system time zone
SYS_VERSION='${release_version}'                             # System version
SYS_ENVIRONMENT='development'                    # System environment
LOG_FORMAT='json'                               # Log format (text, json, default=json)
LOG_LEVEL='info'                                # Log level (debug, info, warn, error, fatal, default=info)

API_PORT='9979'                                 # API Container PORT
API_LOGGER='1'                                  # Log all requests; 0 keeps server errors and degraded responses
CORS_ORIGINS='*'                                # CORS allowed origins
API_TRUSTED_PROXIES=''                          # Reverse proxy IPs/CIDRs, comma separated (empty = no proxy)
API_PROXY_HEADER='X-Real-IP'                    # Client IP header, must be overwritten by the proxy
API_RATE_LIMIT_GLOBAL='300'                     # Requests per client IP per minute, all routes (an office behind NAT shares one)
API_RATE_LIMIT_AUTH='10'                        # Requests per client per minute on register, activate and login
API_RATE_LIMIT_PASSWORD='3'                     # Requests per client per minute on password reset
API_READ_TIMEOUT='30s'                          # Server read timeout
API_WRITE_TIMEOUT='30s'                         # Server write timeout
API_IDLE_TIMEOUT='120s'                         # Keep-alive idle timeout
API_REQUEST_TIMEOUT='10s'                       # Work budget of one request
API_SHUTDOWN_TIMEOUT='30s'                      # Wait for requests in flight at shutdown

ACCESS_TOKEN_EXPIRE='50m'                       # Access token expiration (m=min, s=seg, h=hour, default=15m)
REFRESH_TOKEN_EXPIRE='3h'                       # Refresh token expiration (m=min, s=seg, h=hour, default=60m)

ACCESS_TOKEN='${access_token}'                  # Ed25519 key signing access tokens, base64 PEM (openssl genpkey -algorithm ed25519 | base64 -w0) - PRIVATE
REFRESH_TOKEN='${refresh_token}'                # Ed25519 key signing refresh tokens, must differ from ACCESS_TOKEN - PRIVATE
ACCESS_TOKEN_PREVIOUS=''                        # Rotation: the old ACCESS_TOKEN, still accepted; clear it after one refresh lifetime
REFRESH_TOKEN_PREVIOUS=''                       # Rotation: the old REFRESH_TOKEN, same rule

POSTGRES_HOST='postgres'                        # Postgres Container HOST
POSTGRES_PORT='5445'                            # Postgres Container PORT
POSTGRES_USER='root'                            # Postgres owner, used only by migrations
POSTGRES_PASS='${postgres_pass}'                # Postgres owner PASS
POSTGRES_APP_USER='fin_app'            # Role of backend, created by migration 000001
POSTGRES_APP_PASS='${postgres_app_pass}'        # Its PASS, applied by make migrate-up
POSTGRES_MAILER_USER='fin_mailer'       # Outbox-only role, migration 000001
POSTGRES_MAILER_PASS='${postgres_mailer_pass}'   # Applied by make migrate-up
POSTGRES_BASE='fin'                    # Postgres BASE
POSTGRES_MAX_CONNS='40'                         # Postgres pool per process

SMTP_MASTER_KEY='${smtp_master_key}'            # AES-256 KEK (64 hex chars) sealing email tokens in the outbox — changing it makes queued emails undeliverable
SMTP_ENABLED='0'                                # Send email; required in production
SMTP_HOST=''                                    # SMTP host
SMTP_PORT='587'                                 # SMTP port
SMTP_USER=''                                    # SMTP user
SMTP_PASS=''                                    # SMTP password
SMTP_FROM=''                                    # Sender address
SMTP_REPLY_TO=''                                # Reply-To address (optional)
SMTP_SECURITY='none'                            # none, starttls or tls; production needs TLS
SMTP_APP_URL=''                                 # Web app address for links in emails, e.g. https://id.acme.io; required in production

STORAGE_DRIVER='local'                          # local or s3; s3 when more than one backend instance runs
STORAGE_LOCAL_DIR='data/files'                  # Directory for uploaded files (local driver)
STORAGE_S3_ENDPOINT=''                          # S3-compatible endpoint without the bucket, e.g. http://minio:9000
STORAGE_S3_REGION='us-east-1'                   # Bucket region
STORAGE_S3_BUCKET=''                            # Bucket name
STORAGE_S3_ACCESS_KEY=''                        # Access key id
STORAGE_S3_SECRET_KEY=''                        # Secret access key

BACKUP_DIR='../../backups'                      # Daily database dumps, one per weekday (relative to deploy/docker); they hold all your data - PRIVATE

PLUGGY_BASE_URL='https://api.pluggy.ai'         # Pluggy API address
PLUGGY_CLIENT_ID=''                             # Pluggy client id (dashboard.pluggy.ai); empty disables Open Finance sync
PLUGGY_CLIENT_SECRET=''                         # Pluggy client secret - PRIVATE
PLUGGY_WEBHOOK_URL=''                           # Public https URL of /v1/webhooks/pluggy; empty: no webhook, the daily job syncs
PLUGGY_WEBHOOK_SECRET=''                        # 16+ chars, sent as ?token= on the webhook URL

AI_PROVIDER=''                                  # Assistant: claude (subscription, production), openai (any OpenAI-compatible API, e.g. Ollama) or empty to disable
AI_MODEL=''                                     # claude: opus, sonnet or a full id (empty = CLI default); openai: the model name, e.g. llama3.1
AI_TIMEOUT='120s'                               # Limit of one assistant call
AI_CLAUDE_BIN='claude'                          # Claude Code binary
CLAUDE_CODE_OAUTH_TOKEN=''                      # Token from claude setup-token (1 year, uses your subscription) - PRIVATE
AI_OPENAI_URL='http://localhost:11434/v1'       # OpenAI-compatible base URL; Ollama from a container: http://host.docker.internal:11434/v1
AI_OPENAI_KEY=''                                # API key of that provider; Ollama ignores it - PRIVATE

REGISTRATION_TOKEN_TTL='24h'                      # Registration token time-to-live
PASSWORD_RESET_TOKEN_TTL='2h'                   # Password reset token time-to-live
AUTH_SESSION_MAX_LIFETIME='720h'                # Login required again after this, however often the session refreshes
AUTH_MAX_FAILED_LOGIN_ATTEMPTS='50'             # Bad passwords before the account locks
AUTH_ACCOUNT_LOCK_DURATION='15m'                # How long the lock lasts
AUTH_LOGIN_ATTEMPT_EMAIL_WINDOW='15m'           # Window of the account and client+account limits
AUTH_LOGIN_ATTEMPT_IP_WINDOW='1m'               # Window of the client limit
AUTH_LOGIN_ATTEMPT_IP_MAX='20'                  # Failures per client, per window
AUTH_EMAIL_TOKEN_COOLDOWN='60s'                 # Minimum gap between emails of one purpose
AUTH_LOGIN_ATTEMPT_EMAIL_MAX='50'               # Failures per account, all clients, per window
AUTH_LOGIN_ATTEMPT_PAIR_MAX='5'                 # Failures per client and account, per window
AUTH_EMAIL_TOKEN_HOURLY_MAX='5'                 # Emails per hour, account and purpose (0 = no cap)
AUTH_ALLOW_REGISTRATION='1'                     # Public sign-up; 0 leaves only invitations by an administrator
AUTH_MANUAL_TOKEN_DELIVERY='1'                  # Manual token delivery allowed only in local/dev" >.env
