-- Transactions in these categories become uncategorized (ON DELETE SET NULL).
DELETE FROM app.fin_category
WHERE name IN ('Juros e encargos', 'Seguros', 'Pets', 'Reembolsos', 'Rendimentos', 'Empréstimos');

ALTER TABLE app.fin_account
    DROP COLUMN IF EXISTS last_bill_cents,
    DROP COLUMN IF EXISTS last_bill_due,
    DROP COLUMN IF EXISTS subtype;
