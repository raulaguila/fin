DELETE FROM sys.outbox WHERE purpose = 'alert';
ALTER TABLE sys.outbox DROP CONSTRAINT chk_sys_outbox_purpose;
ALTER TABLE sys.outbox ADD CONSTRAINT chk_sys_outbox_purpose CHECK (purpose IN ('activation', 'password_reset'));
DELETE FROM app.fin_category WHERE name = 'Ignoradas';
DROP TABLE IF EXISTS app.fin_investment;
DROP TABLE IF EXISTS app.fin_goal;
-- Fold each split back into its original before the column goes.
UPDATE app.fin_transaction t SET amount_cents = t.amount_cents + s.total
FROM (SELECT split_of, sum(amount_cents) AS total FROM app.fin_transaction WHERE split_of IS NOT NULL GROUP BY 1) s
WHERE t.id = s.split_of;
DELETE FROM app.fin_transaction WHERE split_of IS NOT NULL;
ALTER TABLE app.fin_transaction DROP COLUMN IF EXISTS split_of;
