CREATE OR REPLACE FUNCTION tracking.fn_stop_item_point(p_tenant_id uuid, p_item jsonb)
 RETURNS geography
 LANGUAGE sql
 STABLE
AS $function$
    SELECT CASE
        WHEN NULLIF(p_item->>'stop_place_id', '') IS NOT NULL THEN (
            SELECT sp.location FROM tracking.stop_places sp
            WHERE sp.id = (p_item->>'stop_place_id')::UUID AND sp.tenant_id = p_tenant_id)
        WHEN p_item->>'latitude' IS NOT NULL AND p_item->>'longitude' IS NOT NULL THEN
            ST_SetSRID(ST_MakePoint((p_item->>'longitude')::DOUBLE PRECISION, (p_item->>'latitude')::DOUBLE PRECISION), 4326)::geography
    END;
$function$;
