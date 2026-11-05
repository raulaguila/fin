-- 000005_outbox_lease_cleanup
-- Lease do mailer (processing volta apos available_at), remocao de tokens em
-- texto puro ja processados, e limpeza diaria das tabelas append-only.

DROP INDEX sys.idx_sys_outbox_pending;

CREATE INDEX idx_sys_outbox_claimable
    ON sys.outbox (available_at)
    WHERE status IN ('pending', 'processing');

COMMENT ON COLUMN sys.outbox.available_at IS
  'Proximo instante elegivel: backoff apos falha ou fim do lease de processing.';
COMMENT ON COLUMN sys.outbox.payload IS
  'Dados do template. token sempre cifrado (AES-256-GCM) e removido ao terminar.';

UPDATE sys.outbox SET payload = payload - 'token' WHERE status IN ('sent', 'failed');

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

COMMENT ON FUNCTION sys.fn_cleanup(INTERVAL) IS
  'Apaga historico vencido. Agendada via pg_cron; sem pg_cron, chamar externamente.';

-- pg_cron so existe na imagem do projeto e so no banco de cron.database_name.
DO $$
BEGIN
  IF current_setting('shared_preload_libraries', true) LIKE '%pg_cron%'
     AND current_setting('cron.database_name', true) = current_database() THEN
    CREATE EXTENSION IF NOT EXISTS pg_cron;
    PERFORM cron.schedule('authenticity_cleanup', '17 3 * * *', 'SELECT sys.fn_cleanup()');
  END IF;
END;
$$;
