DO $$
BEGIN
  IF to_regclass('cron.job') IS NOT NULL THEN
    PERFORM cron.unschedule('authenticity_cleanup');
  END IF;
END;
$$;

DROP FUNCTION IF EXISTS sys.fn_cleanup(INTERVAL);

DROP INDEX sys.idx_sys_outbox_claimable;

CREATE INDEX idx_sys_outbox_pending
    ON sys.outbox (available_at)
    WHERE status = 'pending';
