CREATE OR REPLACE FUNCTION purchasing.sp_list_suppliers(p_tenant_id uuid, p_page_size integer, p_offset integer, p_search character varying DEFAULT NULL::character varying, p_status character varying DEFAULT 'active'::character varying)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, tax_id character varying, legal_name character varying)
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
        s.updated_at,
        s.tax_id,
        s.legal_name
    FROM purchasing.suppliers s
    WHERE s.tenant_id = p_tenant_id
      AND (p_status = 'all' OR s.is_active = (p_status = 'active'))
      AND (
          p_search IS NULL OR p_search = ''
          OR s.name ILIKE '%' || p_search || '%'
          OR s.contact_person ILIKE '%' || p_search || '%'
          OR s.email ILIKE '%' || p_search || '%'
          OR s.tax_id ILIKE '%' || p_search || '%'
      )
    ORDER BY s.name, s.id
    LIMIT p_page_size OFFSET p_offset;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_list_suppliers(uuid,integer,integer,character varying,character varying) IS 'A page of suppliers by name, by status (active, inactive, all) and a search over name, contact, email and tax id (PURCH-001).';
