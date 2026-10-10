CREATE OR REPLACE FUNCTION purchasing.sp_get_supplier(p_supplier_id uuid, p_tenant_id uuid)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM purchasing.suppliers s
        WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        s.id,
        s.name,
        s.contact_person,
        s.email,
        s.phone,
        s.address,
        s.payment_terms,
        s.is_active,
        s.created_at,
        s.updated_at
    FROM purchasing.suppliers s
    WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_get_supplier(uuid,uuid) IS 'Retrieves a single supplier by ID scoped to a tenant. Raises supplier.not-found if not found.';
