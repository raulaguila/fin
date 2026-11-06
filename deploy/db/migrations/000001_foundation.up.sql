-- Extension, schemas, updated_at trigger and the role the processes connect with.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE SCHEMA usr;
CREATE SCHEMA sys;
CREATE SCHEMA app;

COMMENT ON SCHEMA usr IS
  'Identity: users, credentials, sessions, tokens, login attempts, roles, permissions.';
COMMENT ON SCHEMA sys IS
  'System infrastructure: email outbox and scheduled cleanup.';
COMMENT ON SCHEMA app IS
  'Application domain: products (example CRUD for RBAC).';

CREATE FUNCTION public.fn_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

COMMENT ON FUNCTION public.fn_set_updated_at() IS
  'BEFORE UPDATE: fills updated_at. The business clock stays in the application.';

-- Only migrations keep the owner. `make migrate-up` gives this role its login
-- and the password from config/.env.
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'authenticity_app') THEN
        CREATE ROLE authenticity_app NOLOGIN;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA usr, sys, app TO authenticity_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA usr, sys, app
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO authenticity_app;
