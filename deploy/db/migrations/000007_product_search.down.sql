DROP INDEX app.idx_app_product_cursor;
DROP INDEX app.idx_app_product_name_trgm;
-- pg_trgm may be used by other applications; keep the shared extension.
