DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron')
     AND EXISTS (SELECT 1 FROM cron.job WHERE jobname = 'fin_finance_daily') THEN
    PERFORM cron.unschedule('fin_finance_daily');
  END IF;
END;
$$;

DROP TABLE IF EXISTS app.fin_alert;
DROP TABLE IF EXISTS app.fin_rule;
DROP TABLE IF EXISTS app.fin_transaction;
DROP TABLE IF EXISTS app.fin_category;
DROP TABLE IF EXISTS app.fin_account;
DROP TABLE IF EXISTS app.fin_connection;
