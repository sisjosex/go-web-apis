CREATE OR REPLACE FUNCTION tracking.sp_list_trip_events(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(id uuid, type character varying, payload jsonb, created_at timestamp with time zone, created_by_name character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tracking.trips t WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id) THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        e.id,
        CAST(e.type AS VARCHAR),
        e.payload,
        e.created_at,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR)
    FROM tracking.trip_events e
    LEFT JOIN auth.users u ON u.id = e.created_by AND u.deleted_at IS NULL
    WHERE e.trip_id = p_trip_id
    ORDER BY e.created_at, e.id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_trip_events(uuid,uuid) IS 'A tenant''s trip''s events oldest first, each with who made it; raises trip.not-found (TRACK-004 D3)';
