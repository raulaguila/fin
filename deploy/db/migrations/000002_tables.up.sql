-- 000002_tables
-- Tabelas usr + catalogo usr_resource + view. Seed so de usr_user_status.
-- Papeis: 000003. Outbox/products: 000004.

-- ---------------------------------------------------------------------------
-- Lookup de status (precisa existir antes de usr_user)
-- ---------------------------------------------------------------------------
CREATE TABLE usr.usr_user_status (
    code              TEXT        PRIMARY KEY,
    description       TEXT        NOT NULL DEFAULT '',
    is_login_allowed  BOOLEAN     NOT NULL,
    sort_order        INT         NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_usr_user_status_code_lower CHECK (code = lower(code))
);

INSERT INTO usr.usr_user_status (code, description, is_login_allowed, sort_order)
VALUES
    ('pending',  'Cadastro sem prova de caixa; sem senha',              false, 10),
    ('active',   'Ativado; unico status que autentica',                 true,  20),
    ('expired',  'Token de ativacao venceu sem uso; sem senha',         false, 30),
    ('locked',   'Lockout temporario (locked_until na credencial)',     false, 40),
    ('disabled', 'Desligado por operador',                              false, 50)
ON CONFLICT (code) DO NOTHING;

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
    CONSTRAINT fk_usr_user_status
        FOREIGN KEY (status) REFERENCES usr.usr_user_status (code)
        ON UPDATE CASCADE ON DELETE RESTRICT
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
-- Session (unidade de logout + refresh vigente). Tokens de e-mail: usr_token.
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
            'expired'
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
-- Token SMTP (activation | password_reset). Sem session_id. Sem refresh.
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
    CONSTRAINT chk_usr_token_purpose CHECK (purpose IN ('activation', 'password_reset')
    ),
    CONSTRAINT chk_usr_token_attempts_nonneg CHECK (attempts >= 0)
);

CREATE UNIQUE INDEX uni_usr_token_hash ON usr.usr_token (token_hash);
CREATE UNIQUE INDEX uni_usr_token_active_activation ON usr.usr_token (user_id) WHERE purpose = 'activation' AND used_at IS NULL;
CREATE UNIQUE INDEX uni_usr_token_active_password_reset ON usr.usr_token (user_id) WHERE purpose = 'password_reset' AND used_at IS NULL;

-- ---------------------------------------------------------------------------
-- Login attempt (append-only, sem FK de user)
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
            'account_expired',
            'rate_limited_ip',
            'rate_limited_email'
        )
    ),
    CONSTRAINT chk_usr_login_attempt_success_consistency CHECK (
        (success = true  AND failure_kind IS NULL)
        OR (success = false AND failure_kind IS NOT NULL)
    )
);

CREATE INDEX idx_usr_login_attempt_email_created ON usr.usr_login_attempt (email, created_at DESC);
CREATE INDEX idx_usr_login_attempt_ip_created ON usr.usr_login_attempt (ip_address, created_at DESC) WHERE ip_address IS NOT NULL;
CREATE INDEX idx_usr_login_attempt_created ON usr.usr_login_attempt (created_at);

-- ---------------------------------------------------------------------------
-- RBAC B+C: catalogo usr_resource + role + (resource x actions INT4).
-- Sem FK de resource (permite sentinela *); API so aceita catalogo ou *.
--
-- Action bits CRUD (INT4, CHECK >= 0; bit 31 e o sinal, nao usar):
--   1 << 0  create  = 1
--   1 << 1  read    = 2
--   1 << 2  update  = 4
--   1 << 3  delete  = 8
-- Bits 4-30 livres. Nao reutilizar indice.
-- resource='*' e sentinela; a view nao expande; a aplicacao aplica.
-- is_superuser: bypass no Go (EXISTS role enabled); fora da view.
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
    CONSTRAINT pk_usr_role_permission PRIMARY KEY (role_id, resource),
    CONSTRAINT fk_usr_role_permission_role
        FOREIGN KEY (role_id) REFERENCES usr.usr_role (id) ON DELETE CASCADE,
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

-- ---------------------------------------------------------------------------
-- View: OR das actions por resource nos papeis enabled (nao expande '*')
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW usr.vw_user_effective_permissions AS
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
-- updated_at
-- ---------------------------------------------------------------------------
CREATE TRIGGER trg_usr_user_status_set_updated_at
    BEFORE UPDATE ON usr.usr_user_status
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

CREATE TRIGGER trg_usr_user_set_updated_at
    BEFORE UPDATE ON usr.usr_user
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

CREATE TRIGGER trg_usr_credential_set_updated_at
    BEFORE UPDATE ON usr.usr_credential
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

CREATE TRIGGER trg_usr_role_set_updated_at
    BEFORE UPDATE ON usr.usr_role
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

-- ---------------------------------------------------------------------------
-- COMMENT ON
-- ---------------------------------------------------------------------------
COMMENT ON TABLE usr.usr_user_status IS
  'Lookup do ciclo de vida da conta. is_login_allowed e a fonte da verdade do login.';
COMMENT ON COLUMN usr.usr_user_status.is_login_allowed IS
  'True somente em active. Qualquer outro code recusa autenticacao.';

COMMENT ON TABLE usr.usr_user IS
  'Identidade. pending ate activate; expired se o token de ativacao vencer sem uso.';
COMMENT ON COLUMN usr.usr_user.status IS
  'FK para usr_user_status. Unico active autentica (is_login_allowed).';

COMMENT ON TABLE usr.usr_credential IS
  'Credencial 1:1. Hash NULL ate activate. password_algorithm grava argon2id junto do hash.';
COMMENT ON COLUMN usr.usr_credential.password_hash IS
  'Argon2id. NULL ate POST /activate (ou bootstrap admin).';
COMMENT ON COLUMN usr.usr_credential.password_algorithm IS
  'argon2id quando ha hash. NULL quando ainda nao ha senha.';
COMMENT ON COLUMN usr.usr_credential.failed_login_attempts IS
  'Incrementa em senha errada; zera no sucesso. CHECK >= 0.';
COMMENT ON COLUMN usr.usr_credential.locked_until IS
  'Fim do lockout. NULL = nao bloqueado. status=locked acompanha este instante.';

COMMENT ON TABLE usr.usr_session IS
  'Unidade de logout (sid no JWT) e store do refresh vigente (hash do jti).';
COMMENT ON COLUMN usr.usr_session.revocation_reason IS
  'user_logout | token_reuse | password_changed | expired. Sempre junto de revoked_at.';
COMMENT ON COLUMN usr.usr_session.refresh_token_hash IS
  'SHA-256 do jti atual. UNIQUE. Plaintext nunca e gravado.';
COMMENT ON COLUMN usr.usr_session.previous_refresh_token_hash IS
  'jti da geracao anterior. Apresentar este hash com sessao viva = reuse.';
COMMENT ON COLUMN usr.usr_session.ip_address IS
  'Snapshot no login. Nao e device tracking.';
COMMENT ON COLUMN usr.usr_session.user_agent IS
  'Snapshot no login. Nao e device tracking.';

COMMENT ON TABLE usr.usr_token IS
  'Segredo fora da sessao (SMTP): activation e password_reset. Sem refresh.';
COMMENT ON COLUMN usr.usr_token.purpose IS
  'activation | password_reset. Sem session_id.';
COMMENT ON COLUMN usr.usr_token.token_hash IS
  'SHA-256 do valor claro enviado por e-mail. Plaintext nunca e gravado.';
COMMENT ON COLUMN usr.usr_token.attempts IS
  'Recusas apos lookup por hash (expirado/usado). Na 5a, marca used_at.';

COMMENT ON TABLE usr.usr_login_attempt IS
  'Append-only de cada login, inclusive e-mail inexistente. Sem FK para usr_user.';
COMMENT ON COLUMN usr.usr_login_attempt.failure_kind IS
  'Interno. O HTTP nao ecoa este valor (invalid_credentials / account_locked).';

COMMENT ON TABLE usr.usr_resource IS
  'Catalogo de nomes de resource (B+C). * nao entra. API lista daqui.';
COMMENT ON COLUMN usr.usr_resource.code IS
  'Tipo (users, roles, permissions, products). Nao e UUID de instancia.';

COMMENT ON TABLE usr.usr_role IS
  'Papel. Permissoes vivem em usr_role_permission (resource x actions), nao em TEXT[].';
COMMENT ON COLUMN usr.usr_role.is_default IS
  'No maximo um true (indice parcial). Associado no activate.';
COMMENT ON COLUMN usr.usr_role.is_superuser IS
  'Bypass de autorizacao no Go. Seed admin=true. Nao entra na view de permissions.';

COMMENT ON TABLE usr.usr_role_permission IS
  'Uma mascara INT4 por resource no papel. Catalogo em usr_resource; * e sentinela.';
COMMENT ON COLUMN usr.usr_role_permission.resource IS
  'Codigo do catalogo ou * (grant-all). Sem FK (permite *). Nao e UUID de instancia.';
COMMENT ON COLUMN usr.usr_role_permission.actions IS
  'Bits CRUD: create=1 read=2 update=4 delete=8. CHECK >= 0.';

COMMENT ON TABLE usr.usr_user_role IS
  'N:N user x role. DELETE de role com users falha (ON DELETE RESTRICT).';

COMMENT ON VIEW usr.vw_user_effective_permissions IS
  'bit_or(actions) por user+resource nos papeis enabled. Nao expande *. Superuser fora da view.';
