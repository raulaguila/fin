ALTER DEFAULT PRIVILEGES IN SCHEMA usr, sys, app
    REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM authenticity_app;
REVOKE USAGE ON SCHEMA usr, sys, app FROM authenticity_app;
DROP ROLE IF EXISTS authenticity_app;

DROP FUNCTION public.fn_set_updated_at();

DROP SCHEMA app;
DROP SCHEMA sys;
DROP SCHEMA usr;

DROP EXTENSION citext;
