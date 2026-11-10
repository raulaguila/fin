-- Background jobs for any module. A use case enqueues a row in its own
-- transaction, so the job exists exactly when the change it follows commits.
-- The backend's job worker claims due rows with SKIP LOCKED, the same lease and
-- claim_id fencing as sys.outbox. Email stays in sys.outbox: its mailer role
-- must not read anything else.
CREATE TABLE sys.job (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         TEXT        NOT NULL,
    payload      JSONB       NOT NULL DEFAULT '{}',
    status       TEXT        NOT NULL DEFAULT 'pending',
    attempts     INT         NOT NULL DEFAULT 0,
    max_attempts INT         NOT NULL DEFAULT 10,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    claim_id     UUID        NULL,
    request_id   TEXT        NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ NULL,
    last_error   TEXT        NULL,
    CONSTRAINT chk_sys_job_kind CHECK (char_length(kind) BETWEEN 1 AND 100),
    CONSTRAINT chk_sys_job_status CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    CONSTRAINT chk_sys_job_max_attempts CHECK (max_attempts BETWEEN 1 AND 100)
);

CREATE INDEX idx_sys_job_claimable ON sys.job (available_at) WHERE status IN ('pending', 'processing');

CREATE OR REPLACE FUNCTION sys.fn_cleanup(retention INTERVAL DEFAULT INTERVAL '30 days')
RETURNS void
LANGUAGE sql
AS $$
  DELETE FROM usr.usr_login_attempt WHERE created_at < now() - retention;
  DELETE FROM usr.usr_session WHERE expires_at < now() - retention OR revoked_at < now() - retention;
  DELETE FROM usr.usr_token WHERE expires_at < now() - retention;
  DELETE FROM usr.usr_notification WHERE read_at < now() - retention OR created_at < now() - retention * 3;
  DELETE FROM sys.outbox WHERE status IN ('sent', 'failed') AND processed_at < now() - retention;
  DELETE FROM sys.job WHERE status IN ('done', 'failed') AND processed_at < now() - retention;
  DELETE FROM sys.rate_limit WHERE window_start < now() - INTERVAL '1 minute';
  -- Credential and tokens cascade. A live token means a resumed signup: it stays.
  DELETE FROM usr.usr_user u
   WHERE u.status = 'pending'
     AND u.created_at < now() - retention
     AND NOT EXISTS (SELECT 1 FROM usr.usr_token t WHERE t.user_id = u.id AND t.expires_at >= now());
$$;
