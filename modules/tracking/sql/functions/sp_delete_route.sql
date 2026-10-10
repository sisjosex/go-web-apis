CREATE OR REPLACE FUNCTION tracking.sp_delete_route(p_route_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if route exists
    IF NOT EXISTS (SELECT 1 FROM tracking.routes r WHERE r.id = p_route_id) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Delete the route (CASCADE will handle related data)
    DELETE FROM tracking.routes WHERE id = p_route_id;
    
    RETURN TRUE;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_delete_route(p_tenant_id uuid, p_route_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    deleted_count INT;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.routes r
        INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
        WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.routes r WHERE r.id = p_route_id;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count > 0;
END;
$function$;
