-- 000006_cleanup_unactivated_disable
-- Cadastros nunca ativados saem na limpeza diaria; desativar conta tem motivo
-- proprio de revogacao de sessao.

ALTER TABLE usr.usr_session DROP CONSTRAINT chk_usr_session_revocation_reason;
ALTER TABLE usr.usr_session ADD CONSTRAINT chk_usr_session_revocation_reason CHECK (
    revocation_reason IS NULL
    OR revocation_reason IN (
        'user_logout',
        'token_reuse',
        'password_changed',
        'expired',
        'account_disabled'
    )
);

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
  -- Credencial e tokens caem em cascata. Token vivo = cadastro retomado: fica.
  DELETE FROM usr.usr_user u
   WHERE u.status IN ('pending', 'expired')
     AND u.created_at < now() - retention
     AND NOT EXISTS (
       SELECT 1 FROM usr.usr_token t WHERE t.user_id = u.id AND t.expires_at >= now()
     );
$$;
