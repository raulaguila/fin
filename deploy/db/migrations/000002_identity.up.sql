-- Identity: users, credentials, sessions, email tokens, login attempts, RBAC,
-- preferences and notifications, with the default roles.

-- ---------------------------------------------------------------------------
-- User
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_user (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    email      CITEXT      NOT NULL,
    status     TEXT        NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_usr_user_name_len CHECK (char_length(name) BETWEEN 2 AND 80),
    CONSTRAINT chk_usr_user_status CHECK (
        status IN ('pending', 'active', 'locked', 'disabled')
    )
);

CREATE UNIQUE INDEX uni_usr_user_email ON usr.usr_user (email);
CREATE INDEX idx_usr_user_status_non_active ON usr.usr_user (status) WHERE status <> 'active';

-- ---------------------------------------------------------------------------
-- Credential 1:1
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_credential (
    user_id                UUID        PRIMARY KEY,
    password_hash          TEXT        NULL,
    password_algorithm     TEXT        NULL,
    failed_login_attempts  INT         NOT NULL DEFAULT 0,
    locked_until           TIMESTAMPTZ NULL,
    last_login_at          TIMESTAMPTZ NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_usr_credential_user FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_credential_attempts_nonneg CHECK (failed_login_attempts >= 0),
    CONSTRAINT chk_usr_credential_algorithm CHECK (password_algorithm IS NULL OR password_algorithm = 'argon2id'),
    CONSTRAINT chk_usr_credential_hash_algorithm CHECK (
          (password_hash IS NULL AND password_algorithm IS NULL)
          OR (password_hash IS NOT NULL AND password_algorithm IS NOT NULL)
    )
);

-- ---------------------------------------------------------------------------
-- Session: the unit of logout and the current refresh token
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_session (
    id                            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                       UUID        NOT NULL,
    expires_at                    TIMESTAMPTZ NOT NULL,
    revoked_at                    TIMESTAMPTZ NULL,
    revocation_reason             TEXT        NULL,
    refresh_token_hash            BYTEA       NOT NULL,
    previous_refresh_token_hash   BYTEA       NULL,
    ip_address                    INET        NULL,
    user_agent                    TEXT        NULL,
    created_at                    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_usr_session_user
        FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_session_revocation_reason CHECK (
        revocation_reason IS NULL
        OR revocation_reason IN (
            'user_logout',
            'token_reuse',
            'password_changed',
            'expired',
            'account_disabled'
        )
    ),
    CONSTRAINT chk_usr_session_revocation_consistency CHECK (
        (revoked_at IS NULL AND revocation_reason IS NULL)
        OR (revoked_at IS NOT NULL AND revocation_reason IS NOT NULL)
    ),
    CONSTRAINT chk_usr_session_refresh_hashes_distinct CHECK (
        previous_refresh_token_hash IS NULL
        OR previous_refresh_token_hash <> refresh_token_hash
    )
);

CREATE UNIQUE INDEX uni_usr_session_refresh_token_hash ON usr.usr_session (refresh_token_hash);
CREATE INDEX idx_usr_session_user_id ON usr.usr_session (user_id);
CREATE INDEX idx_usr_session_expires_active ON usr.usr_session (expires_at) WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- Email token (activation | password_reset)
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_token (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL,
    purpose    TEXT        NOT NULL,
    token_hash BYTEA       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    attempts   INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_usr_token_user FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_token_purpose CHECK (purpose IN ('activation', 'password_reset')),
    CONSTRAINT chk_usr_token_attempts_nonneg CHECK (attempts >= 0)
);

CREATE UNIQUE INDEX uni_usr_token_hash ON usr.usr_token (token_hash);
CREATE UNIQUE INDEX uni_usr_token_active_activation ON usr.usr_token (user_id) WHERE purpose = 'activation' AND used_at IS NULL;
CREATE UNIQUE INDEX uni_usr_token_active_password_reset ON usr.usr_token (user_id) WHERE purpose = 'password_reset' AND used_at IS NULL;
CREATE INDEX idx_usr_token_user_id ON usr.usr_token (user_id);

-- ---------------------------------------------------------------------------
-- Login attempt (append-only, no user FK)
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_login_attempt (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    email         CITEXT      NOT NULL,
    ip_address    INET        NULL,
    user_agent    TEXT        NULL,
    success       BOOLEAN     NOT NULL,
    failure_kind  TEXT        NULL,
    CONSTRAINT chk_usr_login_attempt_failure_kind CHECK (
        failure_kind IS NULL
        OR failure_kind IN (
            'bad_password',
            'user_not_found',
            'account_locked',
            'account_disabled',
            'account_pending',
            'rate_limited_ip',
            'rate_limited_email'
        )
    ),
    CONSTRAINT chk_usr_login_attempt_success_consistency CHECK (
        (success = true  AND failure_kind IS NULL)
        OR (success = false AND failure_kind IS NOT NULL)
    )
);

-- The predicate must equal the one in attempt/repository.go, or the planner
-- ignores these two indexes.
CREATE INDEX idx_usr_login_attempt_email_counted
    ON usr.usr_login_attempt (email, created_at)
    WHERE success = false
      AND failure_kind NOT IN ('rate_limited_email', 'rate_limited_ip', 'account_locked');
CREATE INDEX idx_usr_login_attempt_ip_counted
    ON usr.usr_login_attempt (ip_address, created_at)
    WHERE success = false
      AND failure_kind NOT IN ('rate_limited_email', 'rate_limited_ip', 'account_locked');
CREATE INDEX idx_usr_login_attempt_created ON usr.usr_login_attempt (created_at);

-- ---------------------------------------------------------------------------
-- RBAC: resource catalog + role + (resource x actions INT4).
-- Every grant names a catalog row or the sentinel '*', which is not a row.
--
-- Action bits (INT4, CHECK >= 0; bit 31 is the sign, do not use):
--   1 << 0  create  = 1
--   1 << 1  read    = 2
--   1 << 2  update  = 4
--   1 << 3  delete  = 8
-- Bits 4-30 are free. Never reuse an index.
-- The view does not expand '*'; the application applies it.
-- is_superuser bypasses authorization in Go and stays out of the view.
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_resource (
    code         VARCHAR(100) PRIMARY KEY,
    description  TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_usr_resource_code CHECK (code ~ '^[a-z0-9_-]{2,100}$')
);

CREATE TABLE usr.usr_role (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    enabled       BOOLEAN     NOT NULL DEFAULT true,
    is_default    BOOLEAN     NOT NULL DEFAULT false,
    is_superuser  BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_usr_role_name CHECK (name ~ '^[a-z0-9_-]{2,40}$')
);

CREATE UNIQUE INDEX uni_usr_role_name ON usr.usr_role (name);
CREATE UNIQUE INDEX uni_usr_role_one_default ON usr.usr_role (is_default) WHERE is_default;

CREATE TABLE usr.usr_role_permission (
    role_id   UUID         NOT NULL,
    resource  VARCHAR(100) NOT NULL,
    actions   INT          NOT NULL,
    -- NULL for '*', which is how the FK below skips the sentinel.
    catalog_code VARCHAR(100) GENERATED ALWAYS AS (NULLIF(resource, '*')) STORED,
    CONSTRAINT pk_usr_role_permission PRIMARY KEY (role_id, resource),
    CONSTRAINT fk_usr_role_permission_role
        FOREIGN KEY (role_id) REFERENCES usr.usr_role (id) ON DELETE CASCADE,
    CONSTRAINT fk_usr_role_permission_resource
        FOREIGN KEY (catalog_code) REFERENCES usr.usr_resource (code),
    CONSTRAINT chk_usr_role_permission_resource
        CHECK (resource = '*' OR resource ~ '^[a-z0-9_-]{2,100}$'),
    CONSTRAINT chk_usr_role_permission_actions
        CHECK (actions >= 0)
);

CREATE INDEX idx_usr_role_permission_resource
    ON usr.usr_role_permission (resource);

CREATE TABLE usr.usr_user_role (
    user_id    UUID        NOT NULL,
    role_id    UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_usr_user_role_user FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT fk_usr_user_role_role FOREIGN KEY (role_id) REFERENCES usr.usr_role (id) ON DELETE RESTRICT
);

CREATE INDEX idx_usr_user_role_role_id ON usr.usr_user_role (role_id);

CREATE VIEW usr.vw_user_effective_permissions AS
SELECT
    ur.user_id,
    rp.resource,
    bit_or(rp.actions) AS actions
FROM usr.usr_user_role ur
JOIN usr.usr_role r
  ON r.id = ur.role_id
 AND r.enabled = true
JOIN usr.usr_role_permission rp
  ON rp.role_id = r.id
GROUP BY ur.user_id, rp.resource;

-- ---------------------------------------------------------------------------
-- Preferences and notifications
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_preference (
    user_id    UUID NOT NULL PRIMARY KEY,
    theme      VARCHAR(10) NOT NULL DEFAULT 'system',
    language   VARCHAR(5) NOT NULL DEFAULT 'pt-BR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_usr_preference_user
        FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_preference_theme CHECK (theme IN ('system', 'light', 'dark'))
);

CREATE TABLE usr.usr_notification (
    id         UUID NOT NULL PRIMARY KEY,
    user_id    UUID NOT NULL,
    level      VARCHAR(10) NOT NULL,
    title      VARCHAR(120) NOT NULL,
    message    TEXT NOT NULL,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT fk_usr_notification_user
        FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_notification_level CHECK (level IN ('info', 'warning', 'critical'))
);

CREATE INDEX idx_usr_notification_user_created ON usr.usr_notification (user_id, created_at DESC);
CREATE INDEX idx_usr_notification_unread ON usr.usr_notification (user_id) WHERE read_at IS NULL;

-- ---------------------------------------------------------------------------
-- updated_at
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_usr_user_set_updated_at
    BEFORE UPDATE ON usr.usr_user
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

CREATE TRIGGER trg_usr_credential_set_updated_at
    BEFORE UPDATE ON usr.usr_credential
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

CREATE TRIGGER trg_usr_role_set_updated_at
    BEFORE UPDATE ON usr.usr_role
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

CREATE TRIGGER trg_usr_preference_set_updated_at
    BEFORE UPDATE ON usr.usr_preference
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

-- ---------------------------------------------------------------------------
-- Seed: resource catalog and default roles. admin only gets '*'.
-- ---------------------------------------------------------------------------
INSERT INTO usr.usr_resource (code, description)
VALUES
    ('users',       'Accounts and profile'),
    ('roles',       'RBAC roles'),
    ('permissions', 'Resources and action masks');

INSERT INTO usr.usr_role (name, enabled, is_default, is_superuser)
VALUES
    ('user',  true, true,  false),
    ('admin', true, false, true);

INSERT INTO usr.usr_role_permission (role_id, resource, actions)
SELECT id, '*', 2147483647 -- every positive INT4 bit
FROM usr.usr_role
WHERE name = 'admin';

-- ---------------------------------------------------------------------------
-- Comments
-- ---------------------------------------------------------------------------
COMMENT ON TABLE usr.usr_user IS
  'Identity. pending until activation. disabled stays until someone enables the account.';
COMMENT ON COLUMN usr.usr_user.status IS
  'pending | active | locked | disabled. Codes match domain.UserStatus*.';

COMMENT ON TABLE usr.usr_credential IS
  'Credential 1:1. Hash is NULL until activation.';
COMMENT ON COLUMN usr.usr_credential.password_hash IS
  'Argon2id. NULL until POST /activate or admin bootstrap.';
COMMENT ON COLUMN usr.usr_credential.password_algorithm IS
  'argon2id when a hash exists. NULL otherwise.';
COMMENT ON COLUMN usr.usr_credential.failed_login_attempts IS
  'Grows on a wrong password; back to zero on success.';
COMMENT ON COLUMN usr.usr_credential.locked_until IS
  'End of the lockout. NULL = not locked. status=locked follows this instant.';

COMMENT ON TABLE usr.usr_session IS
  'Unit of logout (sid in the JWT) and store of the current refresh token (jti hash).';
COMMENT ON COLUMN usr.usr_session.revocation_reason IS
  'user_logout | token_reuse | password_changed | expired | account_disabled. Always set with revoked_at.';
COMMENT ON COLUMN usr.usr_session.refresh_token_hash IS
  'SHA-256 of the current jti. Plaintext is never stored.';
COMMENT ON COLUMN usr.usr_session.previous_refresh_token_hash IS
  'jti of the previous generation. Presenting it on a live session counts as reuse.';
COMMENT ON COLUMN usr.usr_session.ip_address IS
  'Snapshot at login. Not device tracking.';
COMMENT ON COLUMN usr.usr_session.user_agent IS
  'Snapshot at login. Not device tracking.';

COMMENT ON TABLE usr.usr_token IS
  'Secrets sent by email: activation and password_reset.';
COMMENT ON COLUMN usr.usr_token.token_hash IS
  'SHA-256 of the value sent by email. Plaintext is never stored.';
COMMENT ON COLUMN usr.usr_token.attempts IS
  'Refusals after a hash lookup (expired or used). The 5th sets used_at.';

COMMENT ON TABLE usr.usr_login_attempt IS
  'Append-only record of every login, including unknown emails. No FK to usr_user.';
COMMENT ON COLUMN usr.usr_login_attempt.failure_kind IS
  'Internal. HTTP never echoes this value.';

COMMENT ON TABLE usr.usr_resource IS
  'Catalog of resource names, listed by the API and enforced on grants. * stays out.';
COMMENT ON COLUMN usr.usr_resource.code IS
  'Resource type (users, roles, permissions, products). Not an instance UUID.';

COMMENT ON TABLE usr.usr_role IS
  'Role. Permissions live in usr_role_permission.';
COMMENT ON COLUMN usr.usr_role.is_default IS
  'At most one true (partial index). Assigned at activation.';
COMMENT ON COLUMN usr.usr_role.is_superuser IS
  'Bypasses authorization in Go. Stays out of the permissions view.';

COMMENT ON TABLE usr.usr_role_permission IS
  'One INT4 mask per resource in the role.';
COMMENT ON COLUMN usr.usr_role_permission.resource IS
  'Catalog code or * (grant-all).';
COMMENT ON COLUMN usr.usr_role_permission.actions IS
  'Bits: create=1 read=2 update=4 delete=8.';

COMMENT ON TABLE usr.usr_user_role IS
  'N:N user x role. Deleting a role with users fails (ON DELETE RESTRICT).';

COMMENT ON VIEW usr.vw_user_effective_permissions IS
  'bit_or(actions) per user and resource over enabled roles. Does not expand *.';

COMMENT ON TABLE usr.usr_preference IS
  'Interface choices. Without a row, the application uses the defaults.';

COMMENT ON TABLE usr.usr_notification IS
  'Security notices. Read ones leave after the retention; unread ones after three times it.';
