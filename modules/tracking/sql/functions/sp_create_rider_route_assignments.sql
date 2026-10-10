CREATE OR REPLACE FUNCTION tracking.sp_create_rider_route_assignments(p_tenant_id uuid, p_items jsonb)
 RETURNS TABLE(id uuid, rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, direction character varying, days_of_week smallint, pickup_stop_place_id uuid, pickup_stop_name character varying, dropoff_stop_place_id uuid, dropoff_stop_name character varying, valid_from date, valid_until date, created_at timestamp without time zone, updated_at timestamp without time zone, warnings jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_ids   UUID[] := '{}';
    v_item  JSONB;
    v_index INT := -1;
    v_legs  VARCHAR;
    v_dir   VARCHAR;
    v_pair  UUID;
    v_base  JSONB;
    v_twin  JSONB;
    v_this  BOOLEAN;
BEGIN
    BEGIN
        FOR v_item IN SELECT e FROM jsonb_array_elements(COALESCE(p_items, '[]'::jsonb)) e LOOP
            v_index := v_index + 1;
            v_legs := COALESCE(NULLIF(v_item->>'legs', ''), 'both');
            IF v_legs NOT IN ('both', 'outbound', 'return') THEN
                RAISE EXCEPTION 'assignment.legs' USING ERRCODE = 'P0001';
            END IF;
            v_base := v_item - 'legs';
            v_dir := NULL;
            v_pair := NULL;
            SELECT r.direction, r.paired_route_id INTO v_dir, v_pair
            FROM tracking.routes r WHERE r.id = NULLIF(v_base->>'route_id', '')::UUID;

            IF v_pair IS NULL THEN
                v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_base);
                CONTINUE;
            END IF;

            v_this := v_legs = 'both'
                OR (v_legs = 'outbound' AND v_dir = 'outbound')
                OR (v_legs = 'return' AND v_dir = 'inbound');
            v_twin := (v_base - 'pickup_stop_place_id' - 'dropoff_stop_place_id')
                || jsonb_build_object('route_id', v_pair)
                || CASE WHEN v_base ? 'dropoff_stop_place_id'
                       THEN jsonb_build_object('pickup_stop_place_id', v_base->'dropoff_stop_place_id') ELSE '{}'::jsonb END
                || CASE WHEN v_base ? 'pickup_stop_place_id'
                       THEN jsonb_build_object('dropoff_stop_place_id', v_base->'pickup_stop_place_id') ELSE '{}'::jsonb END;
            IF v_this THEN
                v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_base);
            END IF;
            IF v_legs = 'both' OR NOT v_this THEN
                v_ids := v_ids || tracking.fn_assignment_put(p_tenant_id, NULL, v_twin);
            END IF;
        END LOOP;
    EXCEPTION WHEN raise_exception THEN
        RAISE EXCEPTION '%', SQLERRM USING ERRCODE = 'P0001', DETAIL = 'index=' || v_index;
    END;

    RETURN QUERY
    SELECT a.*, tracking.fn_assignment_warnings(
        p_tenant_id, ARRAY(SELECT DISTINCT r.route_id FROM tracking.rider_route_assignments r WHERE r.id = ANY(v_ids))
    )
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, v_ids) a
    ORDER BY array_position(v_ids, a.id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_rider_route_assignments(uuid,jsonb) IS 'Creates every assignment of p_items in one transaction or none, naming the refused item as DETAIL index=<n>; on a paired route an item''s legs (both by default, outbound, return) also write its twin, pickup and drop-off swapped (TRACK-038 D1); returns them with the capacity warnings of the routes touched (TRACK-009)';
