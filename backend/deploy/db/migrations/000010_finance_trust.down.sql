DROP TABLE IF EXISTS sys.backup_run;
ALTER TABLE app.fin_account
    DROP COLUMN IF EXISTS check_since,
    DROP COLUMN IF EXISTS check_sum_cents,
    DROP COLUMN IF EXISTS check_balance_cents;
DROP TABLE IF EXISTS app.fin_undo_row;
DROP TABLE IF EXISTS app.fin_undo;
UPDATE app.fin_transaction SET category_source = 'auto' WHERE category_source = 'pair';
ALTER TABLE app.fin_transaction DROP CONSTRAINT chk_app_fin_transaction_source;
ALTER TABLE app.fin_transaction ADD CONSTRAINT chk_app_fin_transaction_source
    CHECK (category_source IN ('auto', 'rule', 'user'));
