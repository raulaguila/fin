DROP OWNED BY authenticity_app, authenticity_mailer;
DROP ROLE authenticity_app, authenticity_mailer;

DROP FUNCTION public.fn_set_updated_at();
DROP SCHEMA app, sys, usr;
DROP EXTENSION pg_trgm, citext;
