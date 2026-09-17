-- 000004_sys_outbox_app_products
-- Schema sys: outbox de e-mail. Schema app: produto exemplo (RBAC).

CREATE SCHEMA IF NOT EXISTS sys;
CREATE SCHEMA IF NOT EXISTS app;

COMMENT ON SCHEMA sys IS
  'Infraestrutura de sistema: outbox de e-mail (e futuros jobs).';
COMMENT ON SCHEMA app IS
  'Dominio de aplicacao: products (exemplo CRUD para RBAC).';

-- ---------------------------------------------------------------------------
-- Outbox (activation | password_reset | ...). Mailer: SKIP LOCKED.
-- ---------------------------------------------------------------------------
CREATE TABLE sys.outbox (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    purpose       TEXT         NOT NULL,
    payload       JSONB        NOT NULL,
    status        TEXT         NOT NULL DEFAULT 'pending',
    attempts      INT          NOT NULL DEFAULT 0,
    available_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at  TIMESTAMPTZ  NULL,
    last_error    TEXT         NULL,
    CONSTRAINT chk_sys_outbox_purpose CHECK (
        purpose IN ('activation', 'password_reset')
    ),
    CONSTRAINT chk_sys_outbox_status CHECK (
        status IN ('pending', 'processing', 'sent', 'failed')
    ),
    CONSTRAINT chk_sys_outbox_attempts_nonneg CHECK (attempts >= 0)
);

CREATE INDEX idx_sys_outbox_pending
    ON sys.outbox (available_at)
    WHERE status = 'pending';

CREATE INDEX idx_sys_outbox_purpose_created
    ON sys.outbox (purpose, created_at DESC);

COMMENT ON TABLE sys.outbox IS
  'Eventos de e-mail. Backend INSERT pending; mailer SELECT FOR UPDATE SKIP LOCKED.';
COMMENT ON COLUMN sys.outbox.purpose IS
  'activation | password_reset. Expansivel com CHECK novo.';
COMMENT ON COLUMN sys.outbox.payload IS
  'Dados para o template (to, token ref, locale, ...). Sem plaintext de senha.';
COMMENT ON COLUMN sys.outbox.status IS
  'pending | processing | sent | failed.';
COMMENT ON COLUMN sys.outbox.available_at IS
  'Proximo instante elegivel (backoff apos falha).';

-- ---------------------------------------------------------------------------
-- Product (modulo exemplo para exercitar permissions)
-- ---------------------------------------------------------------------------
CREATE TABLE app.product (
    id         UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT           NOT NULL,
    price      NUMERIC(12, 2) NOT NULL,
    enabled    BOOLEAN        NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_product_name_len CHECK (char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT chk_app_product_price_nonneg CHECK (price >= 0)
);

CREATE INDEX idx_app_product_enabled ON app.product (enabled);

CREATE TRIGGER trg_app_product_set_updated_at
    BEFORE UPDATE ON app.product
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

COMMENT ON TABLE app.product IS
  'Modulo exemplo CRUD (name, price, enabled) para testar RBAC products:* .';
