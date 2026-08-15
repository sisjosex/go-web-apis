-- Reverse of 20260812000002 — drops the three shared functions.

DROP FUNCTION IF EXISTS inventory.fn_sku_price(UUID, UUID[]);
DROP FUNCTION IF EXISTS inventory.fn_resolve_sku(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS inventory.fn_stock_status(DECIMAL, DECIMAL);
