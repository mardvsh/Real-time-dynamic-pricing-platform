CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    segment TEXT NOT NULL DEFAULT 'regular',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    base_price NUMERIC(12,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS price_rules (
    id BIGSERIAL PRIMARY KEY,
    rule_name TEXT UNIQUE NOT NULL,
    rule_config JSONB NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID,
    product_id BIGINT REFERENCES products(id),
    final_price NUMERIC(12,2) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO products (name, category, base_price)
VALUES
    ('PlayStation 5', 'electronics', 500.00),
    ('AirPods Pro', 'electronics', 250.00),
    ('MacBook Air', 'electronics', 1300.00)
ON CONFLICT DO NOTHING;

INSERT INTO price_rules (rule_name, rule_config)
VALUES
    ('increase_price_if_views_high', '{"threshold":300,"multiplier":1.10}'),
    ('surge_if_views_very_high', '{"threshold":1000,"multiplier":1.20}'),
    ('discount_if_low_demand', '{"threshold":20,"multiplier":0.95}')
ON CONFLICT (rule_name) DO NOTHING;
