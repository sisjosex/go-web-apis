CREATE OR REPLACE FUNCTION tracking.fn_rider_stop_place(p_tenant_id uuid, p_stop_place_id uuid)
 RETURNS uuid
 LANGUAGE sql
 STABLE
AS $function$
    SELECT sp.id FROM tracking.stop_places sp WHERE sp.id = p_stop_place_id AND sp.tenant_id = p_tenant_id;
$function$;
