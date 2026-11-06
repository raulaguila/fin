REVOKE ALL PRIVILEGES ON sys.outbox FROM authenticity_mailer;
REVOKE USAGE ON SCHEMA sys FROM authenticity_mailer;
-- Keep the cluster-level login: another database may still use it.
