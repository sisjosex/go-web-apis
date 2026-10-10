CREATE OR REPLACE FUNCTION tracking.sp_list_document_types(p_tenant_id uuid, p_applies_to character varying DEFAULT NULL::character varying, p_is_active boolean DEFAULT NULL::boolean)
 RETURNS TABLE(id uuid, tenant_id uuid, code character varying, name character varying, applies_to character varying, warn_days_before integer, blocks_service boolean, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        t.id,
        t.tenant_id,
        CAST(t.code AS VARCHAR),
        CAST(t.name AS VARCHAR),
        CAST(t.applies_to AS VARCHAR),
        t.warn_days_before,
        t.blocks_service,
        t.is_active,
        t.created_at,
        t.updated_at
    FROM tracking.document_types t
    WHERE t.tenant_id = p_tenant_id
      -- 'both' answers either subject, so asking for vehicle types returns the vehicle ones and it.
      AND (p_applies_to IS NULL OR p_applies_to = '' OR t.applies_to IN (p_applies_to, 'both'))
      AND (p_is_active IS NULL OR t.is_active = p_is_active)
    ORDER BY t.name ASC, t.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_document_types(uuid,character varying,boolean) IS 'A tenant''s whole document policy alphabetically; p_applies_to narrows to the types one subject kind may carry, which includes the both-kinds ones';
