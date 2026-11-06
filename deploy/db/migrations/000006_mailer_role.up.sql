-- Mail delivery cannot read or change users, passwords, roles or products.
DO $$ BEGIN
 IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticity_mailer') THEN
  CREATE ROLE authenticity_mailer NOLOGIN;
 END IF;
END $$;
GRANT USAGE ON SCHEMA sys TO authenticity_mailer;
GRANT SELECT ON sys.outbox TO authenticity_mailer;
GRANT UPDATE (status, available_at, claim_id, processed_at, last_error, payload, attempts)
 ON sys.outbox TO authenticity_mailer;
