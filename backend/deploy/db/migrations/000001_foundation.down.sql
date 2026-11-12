DROP OWNED BY fin_app, fin_mailer;
DROP ROLE fin_app, fin_mailer;

DROP FUNCTION public.fn_set_updated_at();
DROP SCHEMA app, sys, usr;
DROP EXTENSION pg_trgm, citext;
