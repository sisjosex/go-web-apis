CREATE OR REPLACE FUNCTION purchasing.sp_get_suppliers_count(p_tenant_id uuid, p_search character varying DEFAULT NULL::character varying, p_status character varying DEFAULT 'active'::character varying)
 RETURNS TABLE(total_count integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT CAST(COUNT(*) AS INT) FROM purchasing.suppliers s
    WHERE s.tenant_id = p_tenant_id
      AND (p_status = 'all' OR s.is_active = (p_status = 'active'))
      AND (
          p_search IS NULL OR p_search = ''
          OR s.name ILIKE '%' || p_search || '%'
          OR s.contact_person ILIKE '%' || p_search || '%'
          OR s.email ILIKE '%' || p_search || '%'
          OR s.tax_id ILIKE '%' || p_search || '%'
      );
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_get_suppliers_count(uuid,character varying,character varying) IS 'How many suppliers match a list''s status and search (PURCH-001).';
