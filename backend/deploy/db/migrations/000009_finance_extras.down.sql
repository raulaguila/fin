DROP TABLE IF EXISTS app.fin_net_worth;
DROP TABLE IF EXISTS app.fin_debt;
ALTER TABLE app.fin_category DROP COLUMN IF EXISTS rollover;
DROP INDEX IF EXISTS app.idx_app_fin_transaction_tags;
ALTER TABLE app.fin_transaction
    DROP CONSTRAINT IF EXISTS chk_app_fin_transaction_tags_count,
    DROP CONSTRAINT IF EXISTS chk_app_fin_transaction_note_len,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS note;
