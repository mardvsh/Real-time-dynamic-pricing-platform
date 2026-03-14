INSERT INTO products (name, category, base_price)
SELECT v.name, v.category, v.base_price
FROM (
  VALUES
    ('PlayStation 5', 'electronics', 500.00::numeric),
    ('AirPods Pro', 'electronics', 250.00::numeric),
    ('MacBook Air', 'electronics', 1300.00::numeric),
    ('iPhone 16', 'electronics', 999.00::numeric),
    ('Nintendo Switch OLED', 'electronics', 349.00::numeric),
    ('iPad Air', 'electronics', 699.00::numeric),
    ('Apple Watch SE', 'electronics', 299.00::numeric),
    ('HomePod mini', 'electronics', 129.00::numeric),
    ('Magic Keyboard', 'accessories', 179.00::numeric),
    ('Studio Display', 'electronics', 1599.00::numeric),
    ('Galaxy S24', 'electronics', 899.00::numeric),
    ('Galaxy Buds', 'electronics', 149.00::numeric),
    ('Galaxy Tab S9', 'electronics', 799.00::numeric),
    ('Pixel 9', 'electronics', 799.00::numeric),
    ('Sony WH-1000XM5', 'electronics', 399.00::numeric),
    ('Dell XPS 13', 'electronics', 1199.00::numeric),
    ('Lenovo ThinkPad X1', 'electronics', 1399.00::numeric),
    ('ASUS ROG Ally', 'gaming', 699.00::numeric),
    ('DJI Mini Drone', 'electronics', 549.00::numeric),
    ('GoPro Hero', 'electronics', 399.00::numeric),
    ('Kindle Paperwhite', 'electronics', 169.00::numeric),
    ('Nintendo Switch OLED Neon', 'gaming', 379.00::numeric)
) AS v(name, category, base_price)
WHERE NOT EXISTS (
  SELECT 1 FROM products p WHERE p.name = v.name
);
