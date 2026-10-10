CREATE OR REPLACE FUNCTION tracking.sp_list_route_exceptions(p_tenant_id uuid, p_route_id uuid, p_date_from date, p_date_to date)
 RETURNS TABLE(id uuid, route_id uuid, date_from date, date_to date, kind character varying, payload jsonb, reason character varying, created_by uuid, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        e.id,
        e.route_id,
        e.date_from,
        e.date_to,
        CAST(e.kind AS VARCHAR),
        e.payload,
        CAST(e.reason AS VARCHAR),
        e.created_by,
        e.created_at,
        e.updated_at
    FROM tracking.route_exceptions e
    WHERE e.route_id = p_route_id
      -- Overlap, not containment: an exception that starts before the window and runs into it is
      -- part of what the window looks like.
      AND (p_date_from IS NULL OR e.date_to >= p_date_from)
      AND (p_date_to   IS NULL OR e.date_from <= p_date_to)
    ORDER BY e.date_from ASC, e.created_at ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_route_exceptions(uuid,uuid,date,date) IS 'A route''s exceptions, oldest first, narrowed to the ones overlapping a window; raises tracking.route.not-found outside the tenant';
