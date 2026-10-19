CREATE OR REPLACE FUNCTION tracking.sp_list_rider_access_log(p_tenant_id uuid, p_rider_id uuid, p_from date DEFAULT NULL::date, p_to date DEFAULT NULL::date, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id bigint, user_id uuid, email character varying, first_name character varying, last_name character varying, level character varying, action character varying, reads integer, first_at timestamp with time zone, last_at timestamp with time zone, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, NULL);

    RETURN QUERY
    SELECT
        l.id,
        l.user_id,
        CAST(u.email AS VARCHAR),
        CAST(u.first_name AS VARCHAR),
        CAST(u.last_name AS VARCHAR),
        CAST(l.level AS VARCHAR),
        CAST(l.action AS VARCHAR),
        l.reads,
        l.first_at,
        l.last_at,
        COUNT(*) OVER ()
    FROM tracking.rider_access_log l
    -- A reader whose account was deleted still read the rider; the row stays with the id alone.
    LEFT JOIN auth.users u ON u.id = l.user_id AND u.deleted_at IS NULL
    WHERE l.rider_id = p_rider_id
      AND l.tenant_id = p_tenant_id
      AND (p_from IS NULL OR l.last_at >= p_from)
      AND (p_to IS NULL OR l.first_at < p_to + 1)
    ORDER BY l.last_at DESC, l.id DESC
    LIMIT GREATEST(p_page_size, 1)
    OFFSET (GREATEST(p_page, 1) - 1) * GREATEST(p_page_size, 1);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_access_log(uuid,uuid,date,date,integer,integer) IS 'Who outside the operator read the rider over [p_from, p_to], one row per user, action and hour with the reader''s name, newest first; raises rider.not-found outside the tenant (TRACK-063)';
