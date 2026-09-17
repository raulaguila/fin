-- 000001_foundation
-- Extensao, schema usr, trigger de updated_at. Zero CREATE TABLE.

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS usr;

COMMENT ON SCHEMA usr IS
  'Identity: users, credentials, sessions, tokens, login attempts, roles, resource catalog, permissions';

CREATE OR REPLACE FUNCTION public.fn_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$;

COMMENT ON FUNCTION public.fn_set_updated_at() IS
  'BEFORE UPDATE: preenche updated_at. Clock de negocio continua na aplicacao.';
