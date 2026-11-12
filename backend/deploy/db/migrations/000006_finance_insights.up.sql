-- Account subtype (checking, savings, card) and the last closed card bill, so
-- two accounts of one bank can be told apart and a paid bill is not shown as open.
ALTER TABLE app.fin_account
    ADD COLUMN subtype         TEXT   NOT NULL DEFAULT '',
    ADD COLUMN last_bill_due   DATE   NULL,
    ADD COLUMN last_bill_cents BIGINT NULL;

-- Categories added to the defaults after the first release, for users who
-- already have theirs. A name the user already has is left alone.
INSERT INTO app.fin_category (user_id, name, kind)
SELECT u.user_id, v.name, v.kind
FROM (SELECT DISTINCT user_id FROM app.fin_category) u
CROSS JOIN (VALUES
    ('Juros e encargos', 'expense'),
    ('Seguros', 'expense'),
    ('Pets', 'expense'),
    ('Reembolsos', 'expense'),
    ('Rendimentos', 'income'),
    ('Empréstimos', 'transfer')
) AS v(name, kind)
ON CONFLICT DO NOTHING;
