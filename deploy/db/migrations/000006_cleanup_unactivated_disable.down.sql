CREATE OR REPLACE FUNCTION sys.fn_cleanup(retention INTERVAL DEFAULT INTERVAL '30 days')
RETURNS void
LANGUAGE sql
AS $$
  DELETE FROM usr.usr_login_attempt WHERE created_at < now() - retention;
  DELETE FROM usr.usr_session
   WHERE expires_at < now() - retention OR revoked_at < now() - retention;
  DELETE FROM usr.usr_token
   WHERE expires_at < now() - retention;
  DELETE FROM sys.outbox
   WHERE status IN ('sent', 'failed') AND processed_at < now() - retention;
$$;

UPDATE usr.usr_session SET revocation_reason = 'user_logout'
 WHERE revocation_reason = 'account_disabled';

ALTER TABLE usr.usr_session DROP CONSTRAINT chk_usr_session_revocation_reason;
ALTER TABLE usr.usr_session ADD CONSTRAINT chk_usr_session_revocation_reason CHECK (
    revocation_reason IS NULL
    OR revocation_reason IN (
        'user_logout',
        'token_reuse',
        'password_changed',
        'expired'
    )
);
