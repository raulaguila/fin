-- 000008_account
-- Preferencias e notificacoes do usuario; indice de produto por nome (ordenacao
-- e lista de opcoes).

CREATE TABLE usr.usr_preference (
    user_id    UUID NOT NULL PRIMARY KEY,
    theme      VARCHAR(10) NOT NULL DEFAULT 'system',
    language   VARCHAR(5) NOT NULL DEFAULT 'pt-BR',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_usr_preference_user
        FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_preference_theme CHECK (theme IN ('system', 'light', 'dark'))
);

CREATE TRIGGER trg_usr_preference_set_updated_at
BEFORE UPDATE ON usr.usr_preference
FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

COMMENT ON TABLE usr.usr_preference IS
'Escolhas de interface. Sem linha, a aplicacao usa os defaults.';

CREATE TABLE usr.usr_notification (
    id         UUID NOT NULL PRIMARY KEY,
    user_id    UUID NOT NULL,
    level      VARCHAR(10) NOT NULL,
    title      VARCHAR(120) NOT NULL,
    message    TEXT NOT NULL,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT fk_usr_notification_user
        FOREIGN KEY (user_id) REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    CONSTRAINT chk_usr_notification_level CHECK (level IN ('info', 'warning', 'critical'))
);

CREATE INDEX idx_usr_notification_user_created ON usr.usr_notification (user_id, created_at DESC);
CREATE INDEX idx_usr_notification_unread ON usr.usr_notification (user_id) WHERE read_at IS NULL;

COMMENT ON TABLE usr.usr_notification IS
'Avisos de seguranca ao usuario. Lidas saem apos a retencao; nao lidas apos o triplo.';

CREATE INDEX idx_app_product_name ON app.product (name);

CREATE OR REPLACE FUNCTION sys.fn_cleanup(retention INTERVAL DEFAULT INTERVAL '30 days')
RETURNS void
LANGUAGE sql
AS $$
  DELETE FROM usr.usr_login_attempt WHERE created_at < now() - retention;
  DELETE FROM usr.usr_session
   WHERE expires_at < now() - retention OR revoked_at < now() - retention;
  DELETE FROM usr.usr_token
   WHERE expires_at < now() - retention;
  DELETE FROM usr.usr_notification
   WHERE read_at < now() - retention OR created_at < now() - retention * 3;
  DELETE FROM sys.outbox
   WHERE status IN ('sent', 'failed') AND processed_at < now() - retention;
  -- Credencial e tokens caem em cascata. Token vivo = cadastro retomado: fica.
  DELETE FROM usr.usr_user u
   WHERE u.status IN ('pending', 'expired')
     AND u.created_at < now() - retention
     AND NOT EXISTS (
       SELECT 1 FROM usr.usr_token t WHERE t.user_id = u.id AND t.expires_at >= now()
     );
$$;
