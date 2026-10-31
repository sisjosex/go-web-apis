CREATE OR REPLACE FUNCTION inventory.sp_set_product_status(p_tenant_id uuid, p_product_id uuid, p_status character varying, p_updated_by uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, status character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    UPDATE inventory.products p
    SET status     = p_status,
        updated_by = p_updated_by,
        updated_at = CURRENT_TIMESTAMP
    WHERE p.id = p_product_id
      AND p.tenant_id = p_tenant_id
    RETURNING p.id, CAST(p.status AS VARCHAR);

    IF NOT FOUND THEN
        RAISE EXCEPTION 'product.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_set_product_status(uuid,uuid,character varying,uuid) IS 'Discontinues or reactivates a product of the tenant (INV-019 D1): its movements, lots and sales stay. Raises product.not-found.';
