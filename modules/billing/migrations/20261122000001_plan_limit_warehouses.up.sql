-- INV-025 D5: how many warehouses each plan allows.
INSERT INTO billing.plan_limits (plan, feature, limit_value) VALUES
    ('free', 'inventory_warehouses', 2),
    ('pro', 'inventory_warehouses', 10),
    ('enterprise', 'inventory_warehouses', -1)
ON CONFLICT (plan, feature) DO UPDATE SET limit_value = EXCLUDED.limit_value;
