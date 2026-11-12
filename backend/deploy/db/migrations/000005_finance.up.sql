-- Personal finance: Open Finance connections (Pluggy items), their accounts and
-- transactions, categories with a monthly limit, categorization rules and the
-- alerts already sent. Every row belongs to one user, and every query filters by
-- user_id. Money is BIGINT cents; a negative amount is money leaving the user.

CREATE TABLE app.fin_connection (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    item_id            TEXT        NOT NULL UNIQUE,
    institution        TEXT        NOT NULL DEFAULT '',
    image_url          TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL DEFAULT 'syncing',
    error              TEXT        NOT NULL DEFAULT '',
    consent_expires_at TIMESTAMPTZ NULL,
    last_sync_at       TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_fin_connection_status CHECK (status IN ('syncing', 'ok', 'error', 'login_required'))
);
CREATE INDEX idx_app_fin_connection_user ON app.fin_connection (user_id);

CREATE TABLE app.fin_account (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    connection_id      UUID        NOT NULL REFERENCES app.fin_connection (id) ON DELETE CASCADE,
    external_id        TEXT        NOT NULL UNIQUE,
    type               TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    number             TEXT        NOT NULL DEFAULT '',
    balance_cents      BIGINT      NOT NULL DEFAULT 0,
    credit_limit_cents BIGINT      NULL,
    available_cents    BIGINT      NULL,
    bill_close_date    DATE        NULL,
    bill_due_date      DATE        NULL,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_fin_account_type CHECK (type IN ('BANK', 'CREDIT'))
);
CREATE INDEX idx_app_fin_account_user ON app.fin_account (user_id);
CREATE INDEX idx_app_fin_account_connection ON app.fin_account (connection_id);

CREATE TABLE app.fin_category (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    kind        TEXT        NOT NULL,
    -- Monthly spending limit; 0 means none.
    limit_cents BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_app_fin_category_name UNIQUE (user_id, name),
    CONSTRAINT chk_app_fin_category_name_len CHECK (char_length(name) BETWEEN 1 AND 40),
    CONSTRAINT chk_app_fin_category_kind CHECK (kind IN ('expense', 'income', 'transfer')),
    CONSTRAINT chk_app_fin_category_limit CHECK (limit_cents >= 0)
);

CREATE TABLE app.fin_transaction (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    account_id        UUID        NOT NULL REFERENCES app.fin_account (id) ON DELETE CASCADE,
    external_id       TEXT        NOT NULL UNIQUE,
    date              DATE        NOT NULL,
    description       TEXT        NOT NULL,
    amount_cents      BIGINT      NOT NULL,
    status            TEXT        NOT NULL DEFAULT 'POSTED',
    provider_category TEXT        NOT NULL DEFAULT '',
    category_id       UUID        NULL REFERENCES app.fin_category (id) ON DELETE SET NULL,
    -- auto: from the bank's category; rule: from a user rule; user: set by hand, never overwritten.
    category_source   TEXT        NOT NULL DEFAULT 'auto',
    installment       SMALLINT    NULL,
    installments      SMALLINT    NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_fin_transaction_source CHECK (category_source IN ('auto', 'rule', 'user'))
);
CREATE INDEX idx_app_fin_transaction_user_date ON app.fin_transaction (user_id, date DESC, id);
CREATE INDEX idx_app_fin_transaction_category ON app.fin_transaction (category_id);
CREATE INDEX idx_app_fin_transaction_account ON app.fin_transaction (account_id);
CREATE INDEX idx_app_fin_transaction_description_trgm ON app.fin_transaction USING gin (description gin_trgm_ops);

CREATE TABLE app.fin_rule (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    keyword     TEXT        NOT NULL,
    category_id UUID        NOT NULL REFERENCES app.fin_category (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_app_fin_rule_keyword UNIQUE (user_id, keyword),
    CONSTRAINT chk_app_fin_rule_keyword_len CHECK (char_length(keyword) BETWEEN 2 AND 60)
);

-- One row per alert sent, so a daily check never repeats it.
CREATE TABLE app.fin_alert (
    user_id    UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

CREATE TRIGGER trg_app_fin_connection_set_updated_at BEFORE UPDATE ON app.fin_connection
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();
CREATE TRIGGER trg_app_fin_account_set_updated_at BEFORE UPDATE ON app.fin_account
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();
CREATE TRIGGER trg_app_fin_transaction_set_updated_at BEFORE UPDATE ON app.fin_transaction
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

-- Daily sync and alerts (10:00 UTC). Without pg_cron, POST /v1/finance/connections/:id/sync still works.
DO $$
BEGIN
  IF current_setting('shared_preload_libraries', true) LIKE '%pg_cron%'
     AND current_setting('cron.database_name', true) = current_database() THEN
    PERFORM cron.schedule('fin_finance_daily', '0 10 * * *',
      $q$INSERT INTO sys.job (kind, payload) VALUES ('finance.daily', '{}')$q$);
  END IF;
END;
$$;
