-- Create sales order management stored procedures
CREATE OR REPLACE FUNCTION sales.sp_create_sales_order(
    p_customer_id UUID,
    p_shipping_address VARCHAR,
    p_notes TEXT,
    p_discount_amount DECIMAL
)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_order_id UUID;
    v_order_number VARCHAR;
BEGIN
    -- Validate customer exists
    IF NOT EXISTS (SELECT 1 FROM sales.customers WHERE id = p_customer_id) THEN
        RAISE EXCEPTION 'customer.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Generate order number
    v_order_number := 'ORD-' || EXTRACT(YEAR FROM NOW()) || '-' || LPAD(NEXTVAL('sales.order_seq')::TEXT, 6, '0');

    -- Insert order
    INSERT INTO sales.sales_orders (
        customer_id, order_number, shipping_address, notes, discount_amount
    ) VALUES (
        p_customer_id, v_order_number, TRIM(p_shipping_address), p_notes, COALESCE(p_discount_amount, 0)
    ) RETURNING sales_orders.id INTO v_order_id;

    -- Return created order
    RETURN QUERY
    SELECT 
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so WHERE so.id = v_order_id;
END;
$$;

-- Get sales order by ID
CREATE OR REPLACE FUNCTION sales.sp_get_sales_order_by_id(p_order_id UUID)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders WHERE id = p_order_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT 
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so WHERE so.id = p_order_id;
END;
$$;

-- Get all sales orders
CREATE OR REPLACE FUNCTION sales.sp_get_all_sales_orders(
    p_limit INT DEFAULT 20,
    p_offset INT DEFAULT 0
)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so 
    ORDER BY so.created_at DESC
    LIMIT p_limit OFFSET p_offset;
END;
$$;

-- Get sales orders by customer
CREATE OR REPLACE FUNCTION sales.sp_get_sales_orders_by_customer(p_customer_id UUID)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so 
    WHERE so.customer_id = p_customer_id
    ORDER BY so.created_at DESC;
END;
$$;

-- Get sales order by order number
CREATE OR REPLACE FUNCTION sales.sp_get_sales_order_by_number(p_order_number VARCHAR)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so 
    WHERE so.order_number = p_order_number;
END;
$$;

-- Update sales order
CREATE OR REPLACE FUNCTION sales.sp_update_sales_order(
    p_order_id UUID,
    p_status VARCHAR,
    p_shipping_address VARCHAR,
    p_notes TEXT,
    p_discount_amount DECIMAL
)
RETURNS TABLE (
    id UUID,
    customer_id UUID,
    order_number VARCHAR,
    status VARCHAR,
    sub_total DECIMAL,
    tax_amount DECIMAL,
    total DECIMAL,
    discount_amount DECIMAL,
    shipping_address VARCHAR,
    notes TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders WHERE id = p_order_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_status IS NOT NULL AND p_status NOT IN ('pending', 'confirmed', 'shipped', 'delivered', 'cancelled') THEN
        RAISE EXCEPTION 'sales-order.invalid-status' USING ERRCODE = 'P0001';
    END IF;

    UPDATE sales.sales_orders SET
        status = COALESCE(p_status, status),
        shipping_address = COALESCE(p_shipping_address, shipping_address),
        notes = COALESCE(p_notes, notes),
        discount_amount = COALESCE(p_discount_amount, discount_amount)
    WHERE id = p_order_id;

    RETURN QUERY
    SELECT 
        CAST(so.id AS UUID),
        CAST(so.customer_id AS UUID),
        CAST(so.order_number AS VARCHAR),
        CAST(so.status AS VARCHAR),
        CAST(so.sub_total AS DECIMAL),
        CAST(so.tax_amount AS DECIMAL),
        CAST(so.total AS DECIMAL),
        CAST(so.discount_amount AS DECIMAL),
        CAST(so.shipping_address AS VARCHAR),
        CAST(so.notes AS TEXT),
        CAST(so.created_at AS TIMESTAMP),
        CAST(so.updated_at AS TIMESTAMP)
    FROM sales.sales_orders so WHERE so.id = p_order_id;
END;
$$;

-- Add order item
CREATE OR REPLACE FUNCTION sales.sp_add_order_item(
    p_order_id UUID,
    p_product_id UUID,
    p_quantity INT
)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    product_id UUID,
    product_sku VARCHAR,
    product_name VARCHAR,
    quantity INT,
    unit_price DECIMAL,
    line_total DECIMAL,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
DECLARE
    v_item_id UUID;
    v_unit_price DECIMAL;
    v_product_sku VARCHAR;
    v_product_name VARCHAR;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM sales.sales_orders WHERE id = p_order_id) THEN
        RAISE EXCEPTION 'sales-order.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_quantity <= 0 THEN
        RAISE EXCEPTION 'order-item.invalid-qty' USING ERRCODE = 'P0001';
    END IF;

    -- Get product info from inventory
    SELECT price, sku, name INTO v_unit_price, v_product_sku, v_product_name
    FROM inventory.products WHERE id = p_product_id;

    IF v_unit_price IS NULL THEN
        RAISE EXCEPTION 'order-item.product-not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Insert order item
    INSERT INTO sales.order_items (
        order_id, product_id, product_sku, product_name, quantity, unit_price
    ) VALUES (
        p_order_id, p_product_id, v_product_sku, v_product_name, p_quantity, v_unit_price
    ) RETURNING order_items.id INTO v_item_id;

    RETURN QUERY
    SELECT 
        CAST(oi.id AS UUID),
        CAST(oi.order_id AS UUID),
        CAST(oi.product_id AS UUID),
        CAST(oi.product_sku AS VARCHAR),
        CAST(oi.product_name AS VARCHAR),
        CAST(oi.quantity AS INT),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi WHERE oi.id = v_item_id;
END;
$$;

-- Get order items
CREATE OR REPLACE FUNCTION sales.sp_get_order_items(p_order_id UUID)
RETURNS TABLE (
    id UUID,
    order_id UUID,
    product_id UUID,
    product_sku VARCHAR,
    product_name VARCHAR,
    quantity INT,
    unit_price DECIMAL,
    line_total DECIMAL,
    created_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        CAST(oi.id AS UUID),
        CAST(oi.order_id AS UUID),
        CAST(oi.product_id AS UUID),
        CAST(oi.product_sku AS VARCHAR),
        CAST(oi.product_name AS VARCHAR),
        CAST(oi.quantity AS INT),
        CAST(oi.unit_price AS DECIMAL),
        CAST(oi.line_total AS DECIMAL),
        CAST(oi.created_at AS TIMESTAMP)
    FROM sales.order_items oi 
    WHERE oi.order_id = p_order_id
    ORDER BY oi.created_at;
END;
$$;

-- Create sequence for order numbering
CREATE SEQUENCE IF NOT EXISTS sales.order_seq START WITH 1000;
