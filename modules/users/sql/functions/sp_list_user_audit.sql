CREATE OR REPLACE FUNCTION users.sp_list_user_audit(p_tenant_id uuid, p_page integer DEFAULT 1, p_limit integer DEFAULT 20, p_search character varying DEFAULT NULL::character varying, p_action character varying DEFAULT NULL::character varying, p_from timestamp with time zone DEFAULT NULL::timestamp with time zone, p_to timestamp with time zone DEFAULT NULL::timestamp with time zone)
 RETURNS TABLE(id uuid, action character varying, target_user_id uuid, target_user_email character varying, performed_by uuid, performed_by_email character varying, metadata jsonb, source character varying, created_at timestamp with time zone, total bigint)
 LANGUAGE plpgsql
AS $function$
DECLARE
    l_offset INT := (p_page - 1) * p_limit;
BEGIN
    RETURN QUERY
    SELECT
        al.id,
        CAST(al.action AS VARCHAR),
        al.target_user_id,
        CAST(tu.email AS VARCHAR),
        al.performed_by,
        CAST(pu.email AS VARCHAR),
        al.metadata,
        CAST(al.source AS VARCHAR),
        al.created_at,
        COUNT(*) OVER () AS total
    FROM users.user_audit_log al
    LEFT JOIN auth.users tu ON tu.id = al.target_user_id
    LEFT JOIN auth.users pu ON pu.id = al.performed_by
    WHERE al.tenant_id = p_tenant_id
      AND (p_action IS NULL OR al.action = p_action)
      AND (p_from IS NULL OR al.created_at >= p_from)
      AND (p_to IS NULL OR al.created_at <= p_to)
      AND (
          p_search IS NULL
          OR CAST(tu.email AS TEXT) ILIKE '%' || p_search || '%'
          OR CAST(pu.email AS TEXT) ILIKE '%' || p_search || '%'
      )
    ORDER BY al.created_at DESC
    LIMIT p_limit
    OFFSET l_offset;
END;
$function$;

COMMENT ON FUNCTION users.sp_list_user_audit(uuid,integer,integer,character varying,character varying,timestamp with time zone,timestamp with time zone) IS 'Returns paginated audit log entries scoped to a tenant with optional filtering';
