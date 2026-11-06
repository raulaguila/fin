-- Example CRUD module that exercises RBAC.

CREATE TABLE app.product (
    id         UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT           NOT NULL,
    price      NUMERIC(12, 2) NOT NULL,
    enabled    BOOLEAN        NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_product_name_len CHECK (char_length(name) BETWEEN 1 AND 120),
    CONSTRAINT chk_app_product_price_nonneg CHECK (price >= 0)
);

CREATE INDEX idx_app_product_enabled ON app.product (enabled);
CREATE INDEX idx_app_product_created ON app.product (created_at DESC);
CREATE INDEX idx_app_product_name ON app.product (name);

CREATE TRIGGER trg_app_product_set_updated_at
    BEFORE UPDATE ON app.product
    FOR EACH ROW EXECUTE FUNCTION public.fn_set_updated_at();

COMMENT ON TABLE app.product IS
  'Example CRUD (name, price, enabled) that exercises the products RBAC resource.';

INSERT INTO usr.usr_resource (code, description)
VALUES ('products', 'Example CRUD module');
