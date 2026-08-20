#!/bin/bash

release_version=$(cat config/version.txt | tr -d "[:space:]")
access_token=$(openssl genrsa 2048 | base64 | tr -d \\n)
refresh_token=$(openssl genrsa 2048 | base64 | tr -d \\n)
# 32 bytes (AES-256) as 64-char hex. Master KEK that wraps SMTP data-encryption
# keys at rest. Regenerating this key on an existing DB makes stored SMTP
# passwords unreadable — rotate only via a re-wrap job.
smtp_master_key=$(openssl rand -hex 32)

echo "TZ='America/Manaus'                             # Set system time zone
SYS_VERSION='${release_version}'                             # System version
SYS_ENVIRONMENT='development'                    # System environment
LOG_FORMAT='text'                               # Log format (text, json, default=text)
LOG_LEVEL='info'                                # Log level (debug, info, warn, error, fatal, default=info)

API_PORT='9978'                                 # API Container PORT
API_LOGGER='1'                                  # API Logger enable
API_SWAGGO='1'                                  # API Swagger enable
API_PREFORK='0'                                 # API enable fiber prefork
CORS_ORIGINS='*'                                # CORS allowed origins

ACCESS_TOKEN_EXPIRE='50m'                       # Access token expiration (m=min, s=seg, h=hour, default=50m)
REFRESH_TOKEN_EXPIRE='3h'                       # Refresh token expiration (m=min, s=seg, h=hour, default=3h)

ACCESS_TOKEN='${access_token}'                  # Token to encode access token - PRIVATE TOKEN
REFRESH_TOKEN='${refresh_token}'                # Token to encode refresh token - PRIVATE TOKEN

POSTGRES_HOST='postgres'                        # Postgres Container HOST
POSTGRES_PORT='5444'                            # Postgres Container PORT
POSTGRES_USER='root'                            # Postgres USER
POSTGRES_PASS='root'                            # Postgres PASS
POSTGRES_BASE='authenticity'                    # Postgres BASE

CACHE_HOST='cache'                              # Cache HOST
CACHE_PORT='6379'                               # Cache PORT
CACHE_USER='default'                            # Cache USER
CACHE_PASS='root'                               # Cache PASS
CACHE_DB='0'                                    # Cache DB
CACHE_TTL='10m'                                 # Cache TTL

SMTP_MASTER_KEY='${smtp_master_key}'            # AES-256 KEK (64 hex chars) wrapping SMTP DEKs at rest — regenerate invalidates stored passwords

REGISTRATION_TOKEN_TTL='24h'                      # Registration token time-to-live
PASSWORD_RESET_TOKEN_TTL='2h'                   # Password reset token time-to-live
AUTH_MANUAL_TOKEN_DELIVERY='1'                  # Manual token delivery allowed only in local/dev" >.env
