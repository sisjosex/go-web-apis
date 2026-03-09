-- Migration: sp_purchase_order_items_operations
-- Module: purchasing
-- Description: Stored procedures for purchase order items operations

-- GET PURCHASE ORDER ITEMS
CREATE OR REPLACE FUNCTION purchasing.sp_get_purchase_order_items(
    p_purchase_order_id UUID
)
RETURNS TABLE(
    id UUID,
    purchase_order_id UUID,
    product_id UUID,
    quantity INT,
    unit_cost DECIMAL,
    line_total DECIMAL,
    received_quantity INT,
    status VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        poi.id,
        poi.purchase_order_id,
        poi.product_id,
        poi.quantity,
        poi.unit_cost,
        poi.line_total,
        poi.received_quantity,
        poi.status,
        poi.created_at,
        poi.updated_at
    FROM purchasing.purchase_order_items poi
    WHERE poi.purchase_order_id = p_purchase_order_id
    ORDER BY poi.created_at;
END;
$$;
