CREATE OR REPLACE FUNCTION tracking.fn_calendar_routes(p_tenant_id uuid, p_calendar_id uuid)
 RETURNS TABLE(route_id uuid)
 LANGUAGE sql
 STABLE
AS $function$
    -- A schedule calendar reaches the routes of the schedules that point at it; a tenant calendar
    -- every route; a client calendar every route of that client (TRACK-076 D1).
    SELECT s.route_id
    FROM tracking.route_schedules s
    INNER JOIN tracking.calendars c ON c.id = s.calendar_id AND c.tenant_id = p_tenant_id
    WHERE s.calendar_id = p_calendar_id
    UNION
    SELECT r.id
    FROM tracking.calendars c
    INNER JOIN tracking.transport_companies tc ON tc.tenant_id = c.tenant_id
    INNER JOIN tracking.routes r
            ON r.company_id = tc.id
           AND (c.scope = 'tenant' OR (c.scope = 'organization' AND r.organization_id = c.organization_id))
    WHERE c.id = p_calendar_id AND c.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION tracking.fn_calendar_routes(uuid,uuid) IS 'The routes a calendar reaches: those whose schedules point at it, every route for the tenant calendar, the client''s routes for an organization calendar (TRACK-076 D1)';
