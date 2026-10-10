CREATE OR REPLACE FUNCTION inventory.sp_get_movement(p_tenant_id uuid, p_movement_id uuid)
 RETURNS TABLE(id uuid, product_id uuid, movement_type character varying, quantity numeric, reference_type character varying, reference_id uuid, unit_cost numeric, notes text, created_by uuid, created_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        m.id,
        m.product_id,
        m.movement_type,
        m.quantity,
        m.reference_type,
        m.reference_id,
        m.unit_cost,
        m.notes,
        m.created_by,
        m.created_at
    FROM inventory.inventory_movements m
    WHERE m.tenant_id = p_tenant_id AND m.id = p_movement_id;
END;
$function$;
