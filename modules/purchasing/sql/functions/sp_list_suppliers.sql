CREATE OR REPLACE FUNCTION purchasing.sp_list_suppliers(p_tenant_id uuid, p_page_size integer, p_offset integer)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
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
    WHERE s.tenant_id = p_tenant_id AND s.is_active = TRUE
    ORDER BY s.created_at DESC
    LIMIT p_page_size OFFSET p_offset;
END;
$function$;
