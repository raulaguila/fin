CREATE OR REPLACE FUNCTION sys.fn_cleanup(retention INTERVAL DEFAULT INTERVAL '30 days')
RETURNS void
LANGUAGE sql
AS $$
  DELETE FROM usr.usr_login_attempt WHERE created_at < now() - retention;
  DELETE FROM usr.usr_session WHERE expires_at < now() - retention OR revoked_at < now() - retention;
  DELETE FROM usr.usr_token WHERE expires_at < now() - retention;
  DELETE FROM usr.usr_notification WHERE read_at < now() - retention OR created_at < now() - retention * 3;
  DELETE FROM sys.outbox WHERE status IN ('sent', 'failed') AND processed_at < now() - retention;
  DELETE FROM sys.rate_limit WHERE window_start < now() - INTERVAL '1 minute';
  -- Credential and tokens cascade. A live token means a resumed signup: it stays.
  DELETE FROM usr.usr_user u
   WHERE u.status = 'pending'
     AND u.created_at < now() - retention
     AND NOT EXISTS (SELECT 1 FROM usr.usr_token t WHERE t.user_id = u.id AND t.expires_at >= now());
$$;

DROP TABLE sys.job;
