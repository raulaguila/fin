-- A split moves part of a transaction to another category: the part is its own
-- row pointing at the original, and the original keeps the rest.
ALTER TABLE app.fin_transaction
    ADD COLUMN split_of UUID NULL REFERENCES app.fin_transaction (id) ON DELETE CASCADE;
CREATE INDEX idx_app_fin_transaction_split_of ON app.fin_transaction (split_of) WHERE split_of IS NOT NULL;

-- Savings goals: progress is the balance of the linked account, or the amount the user types.
CREATE TABLE app.fin_goal (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    target_cents BIGINT      NOT NULL,
    saved_cents  BIGINT      NOT NULL DEFAULT 0,
    account_id   UUID        NULL REFERENCES app.fin_account (id) ON DELETE SET NULL,
    due_date     DATE        NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_fin_goal_name_len CHECK (char_length(name) BETWEEN 1 AND 60),
    CONSTRAINT chk_app_fin_goal_target CHECK (target_cents > 0)
);
CREATE INDEX idx_app_fin_goal_user ON app.fin_goal (user_id);

-- Investments read from the provider at each sync.
CREATE TABLE app.fin_investment (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    connection_id UUID        NOT NULL REFERENCES app.fin_connection (id) ON DELETE CASCADE,
    external_id   TEXT        NOT NULL UNIQUE,
    name          TEXT        NOT NULL,
    type          TEXT        NOT NULL DEFAULT '',
    balance_cents BIGINT      NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_app_fin_investment_user ON app.fin_investment (user_id);
CREATE INDEX idx_app_fin_investment_connection ON app.fin_investment (connection_id);

-- "Ignoradas" is a transfer category: money in it counts nowhere (a duplicate, a refund pair).
INSERT INTO app.fin_category (user_id, name, kind)
SELECT DISTINCT user_id, 'Ignoradas', 'transfer' FROM app.fin_category
ON CONFLICT DO NOTHING;

-- Finance alerts can go out by email too.
ALTER TABLE sys.outbox DROP CONSTRAINT chk_sys_outbox_purpose;
ALTER TABLE sys.outbox ADD CONSTRAINT chk_sys_outbox_purpose CHECK (purpose IN ('activation', 'password_reset', 'alert'));
