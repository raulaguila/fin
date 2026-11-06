-- Email outbox, with the mailer's lease, and the daily cleanup.

CREATE TABLE sys.outbox (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    purpose       TEXT         NOT NULL,
    payload       JSONB        NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'pending',
    attempts      INT          NOT NULL DEFAULT 0,
    available_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at  TIMESTAMPTZ  NULL,
    last_error    TEXT         NULL,
    CONSTRAINT chk_sys_outbox_purpose CHECK (
        purpose IN ('activation', 'password_reset')
    ),
    CONSTRAINT chk_sys_outbox_status CHECK (
        status IN ('pending', 'processing', 'sent', 'failed')
    ),
    CONSTRAINT chk_sys_outbox_attempts_nonneg CHECK (attempts >= 0)
);

CREATE INDEX idx_sys_outbox_claimable
    ON sys.outbox (available_at)
    WHERE status IN ('pending', 'processing');

COMMENT ON TABLE sys.outbox IS
  'Email events. The backend inserts pending rows; the mailer claims them with SKIP LOCKED.';
COMMENT ON COLUMN sys.outbox.purpose IS
  'activation | password_reset.';
COMMENT ON COLUMN sys.outbox.payload IS
  'Template data. The token is always sealed (AES-256-GCM) and removed when finished.';
COMMENT ON COLUMN sys.outbox.status IS
  'pending | processing | sent | failed.';
COMMENT ON COLUMN sys.outbox.available_at IS
  'Next eligible instant: backoff after a failure, or the end of the processing lease.';

CREATE FUNCTION sys.fn_cleanup(retention INTERVAL DEFAULT INTERVAL '30 days')
RETURNS void
LANGUAGE sql
AS $$
  DELETE FROM usr.usr_login_attempt WHERE created_at < now() - retention;
  DELETE FROM usr.usr_session
   WHERE expires_at < now() - retention OR revoked_at < now() - retention;
  DELETE FROM usr.usr_token
   WHERE expires_at < now() - retention;
  DELETE FROM usr.usr_notification
   WHERE read_at < now() - retention OR created_at < now() - retention * 3;
  DELETE FROM sys.outbox
   WHERE status IN ('sent', 'failed') AND processed_at < now() - retention;
  -- Credential and tokens cascade. A live token means a resumed signup: it stays.
  DELETE FROM usr.usr_user u
   WHERE u.status = 'pending'
     AND u.created_at < now() - retention
     AND NOT EXISTS (
       SELECT 1 FROM usr.usr_token t WHERE t.user_id = u.id AND t.expires_at >= now()
     );
$$;

COMMENT ON FUNCTION sys.fn_cleanup(INTERVAL) IS
  'Deletes expired history. Scheduled through pg_cron; without it, call it externally.';

-- pg_cron only exists in the project image, and only in cron.database_name.
DO $$
BEGIN
  IF current_setting('shared_preload_libraries', true) LIKE '%pg_cron%'
     AND current_setting('cron.database_name', true) = current_database() THEN
    CREATE EXTENSION IF NOT EXISTS pg_cron;
    PERFORM cron.schedule('authenticity_cleanup', '17 3 * * *', 'SELECT sys.fn_cleanup()');
  END IF;
END;
$$;
