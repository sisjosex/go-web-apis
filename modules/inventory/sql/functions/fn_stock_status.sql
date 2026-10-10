CREATE OR REPLACE FUNCTION inventory.fn_stock_status(p_quantity numeric, p_reorder_level numeric)
 RETURNS character varying
 LANGUAGE plpgsql
 IMMUTABLE
AS $function$
DECLARE
    v_status VARCHAR;
BEGIN
    IF p_quantity = 0
    THEN
        v_status := 'out_of_stock';
    ELSIF p_quantity < p_reorder_level
    THEN
        v_status := 'critical';
    ELSIF p_quantity < (p_reorder_level * 1.5)
    THEN
        v_status := 'low';
    ELSE
        v_status := 'ok';
    END IF;

    RETURN v_status;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_stock_status(numeric,numeric) IS 'The stock status ladder, previously copy-pasted into four SPs: 0 is out_of_stock, below reorder_level is critical, below reorder_level * 1.5 is low, anything else is ok. A NULL quantity or reorder level falls through to ok, exactly as the inlined ladder did.';
