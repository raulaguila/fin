DO $$
BEGIN
  IF to_regclass('cron.job') IS NOT NULL THEN
    PERFORM cron.unschedule('authenticity_cleanup');
    PERFORM cron.unschedule('authenticity_rate_limit');
  END IF;
END;
$$;

DROP FUNCTION sys.fn_cleanup(INTERVAL);
DROP TABLE sys.rate_limit, sys.outbox;
