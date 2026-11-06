CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_app_product_name_trgm ON app.product USING gin (name gin_trgm_ops);
CREATE INDEX idx_app_product_cursor ON app.product (created_at DESC, id DESC);
