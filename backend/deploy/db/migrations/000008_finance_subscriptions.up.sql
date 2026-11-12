-- Subscriptions and fixed bills the user registers. The amount is what it costs
-- (positive); month NULL means every month, otherwise once a year in that month.
CREATE TABLE app.fin_subscription (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    amount_cents BIGINT      NOT NULL,
    day          SMALLINT    NOT NULL,
    month        SMALLINT    NULL,
    keyword      TEXT        NOT NULL,
    category_id  UUID        NULL REFERENCES app.fin_category (id) ON DELETE SET NULL,
    account_id   UUID        NULL REFERENCES app.fin_account (id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_fin_subscription_name_len CHECK (char_length(name) BETWEEN 1 AND 60),
    CONSTRAINT chk_app_fin_subscription_keyword_len CHECK (char_length(keyword) BETWEEN 2 AND 60),
    CONSTRAINT chk_app_fin_subscription_amount CHECK (amount_cents > 0),
    CONSTRAINT chk_app_fin_subscription_day CHECK (day BETWEEN 1 AND 31),
    CONSTRAINT chk_app_fin_subscription_month CHECK (month BETWEEN 1 AND 12)
);
CREATE INDEX idx_app_fin_subscription_user ON app.fin_subscription (user_id);

-- Charges the detection took for recurring that the user said are not.
CREATE TABLE app.fin_recurring_ignored (
    user_id UUID NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    key     TEXT NOT NULL,
    PRIMARY KEY (user_id, key),
    CONSTRAINT chk_app_fin_recurring_ignored_key_len CHECK (char_length(key) BETWEEN 1 AND 120)
);
