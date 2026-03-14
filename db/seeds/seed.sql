INSERT INTO products (name, category, base_price)
VALUES
    ('iPhone 16', 'electronics', 999.00),
    ('Nintendo Switch OLED', 'electronics', 349.00)
ON CONFLICT DO NOTHING;
