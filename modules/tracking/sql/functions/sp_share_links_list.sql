CREATE OR REPLACE FUNCTION tracking.sp_share_links_list(p_tenant_id uuid, p_trip_id uuid)
 RETURNS TABLE(id uuid, created_by_name character varying, created_via character varying, expires_at timestamp with time zone, created_at timestamp with time zone)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    PERFORM 1 FROM tracking.trips t WHERE t.id = p_trip_id AND t.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        l.id,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        CAST(l.created_via AS VARCHAR),
        l.expires_at,
        l.created_at
    FROM tracking.share_links l
    LEFT JOIN auth.users u ON u.id = l.created_by AND u.deleted_at IS NULL
    WHERE l.tenant_id = p_tenant_id
      AND l.trip_id = p_trip_id
      AND l.revoked_at IS NULL
      AND l.expires_at > now()
    ORDER BY l.created_at DESC, l.id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_share_links_list(uuid,uuid) IS 'A trip''s share links still open, newest first, with who made them; raises trip.not-found (TRACK-060)';
