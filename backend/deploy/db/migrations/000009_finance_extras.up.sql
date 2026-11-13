-- A note and tags the user writes on a transaction. Syncs never touch them.
ALTER TABLE app.fin_transaction
    ADD COLUMN note TEXT   NOT NULL DEFAULT '',
    ADD COLUMN tags TEXT[] NOT NULL DEFAULT '{}',
    ADD CONSTRAINT chk_app_fin_transaction_note_len CHECK (char_length(note) <= 500),
    ADD CONSTRAINT chk_app_fin_transaction_tags_count CHECK (cardinality(tags) <= 10);
CREATE INDEX idx_app_fin_transaction_tags ON app.fin_transaction USING gin (tags);

-- With rollover, what a category did not spend last month adds to this month's
-- limit, and what it overspent comes off it.
ALTER TABLE app.fin_category ADD COLUMN rollover BOOLEAN NOT NULL DEFAULT false;

-- Money lent or borrowed between people. Positive: the person owes the user;
-- negative: the user owes the person. settled_on marks it paid.
CREATE TABLE app.fin_debt (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    person       TEXT        NOT NULL,
    amount_cents BIGINT      NOT NULL,
    description  TEXT        NOT NULL DEFAULT '',
    due_date     DATE        NULL,
    settled_on   DATE        NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_fin_debt_person_len CHECK (char_length(person) BETWEEN 1 AND 60),
    CONSTRAINT chk_app_fin_debt_description_len CHECK (char_length(description) <= 120),
    CONSTRAINT chk_app_fin_debt_amount CHECK (amount_cents <> 0)
);
CREATE INDEX idx_app_fin_debt_user ON app.fin_debt (user_id, settled_on);

-- One picture of the net worth per day: bank balances, investments, card bills.
CREATE TABLE app.fin_net_worth (
    user_id           UUID   NOT NULL REFERENCES usr.usr_user (id) ON DELETE CASCADE,
    day               DATE   NOT NULL,
    banks_cents       BIGINT NOT NULL,
    investments_cents BIGINT NOT NULL,
    cards_cents       BIGINT NOT NULL,
    PRIMARY KEY (user_id, day)
);
