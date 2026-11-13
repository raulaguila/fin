-- Trust in the numbers: transfers matched between the user's own accounts,
-- undo of category changes, a balance check per bank account and the backup log.

-- pair: matched with the opposite movement in another of the user's accounts.
ALTER TABLE app.fin_transaction DROP CONSTRAINT chk_app_fin_transaction_source;
ALTER TABLE app.fin_transaction ADD CONSTRAINT chk_app_fin_transaction_source
    CHECK (category_source IN ('auto', 'rule', 'user', 'pair'));

-- What a bulk change or a rule overwrote, so it can be put back for a while.
CREATE TABLE app.fin_undo (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    rule_id    UUID        NULL REFERENCES app.fin_rule (id) ON DELETE SET NULL,
    -- What the change set: undo leaves alone a row changed again since.
    set_category_id UUID   NULL,
    set_source      TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_app_fin_undo_created ON app.fin_undo (created_at);

CREATE TABLE app.fin_undo_row (
    undo_id         UUID NOT NULL REFERENCES app.fin_undo (id) ON DELETE CASCADE,
    transaction_id  UUID NOT NULL REFERENCES app.fin_transaction (id) ON DELETE CASCADE,
    category_id     UUID NULL,
    category_source TEXT NOT NULL,
    PRIMARY KEY (undo_id, transaction_id)
);
CREATE INDEX idx_app_fin_undo_row_transaction ON app.fin_undo_row (transaction_id);

-- Balance check: the bank balance and the sum of the account's transactions
-- when the check started. Both must move together; a gap is missing data.
ALTER TABLE app.fin_account
    ADD COLUMN check_balance_cents BIGINT      NULL,
    ADD COLUMN check_sum_cents     BIGINT      NULL,
    ADD COLUMN check_since         TIMESTAMPTZ NULL;

-- Written by the backup service after each dump, copy and restore test.
CREATE TABLE sys.backup_run (
    id         BIGSERIAL   PRIMARY KEY,
    kind       TEXT        NOT NULL,
    ok         BOOLEAN     NOT NULL,
    detail     TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_sys_backup_run_kind CHECK (kind IN ('dump', 'copy', 'restore_check'))
);
CREATE INDEX idx_sys_backup_run_kind ON sys.backup_run (kind, created_at DESC);
