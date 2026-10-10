CREATE OR REPLACE FUNCTION tracking.sp_bulk_assign_riders(p_tenant_id uuid, p_route_id uuid, p_rider_ids uuid[], p_days smallint DEFAULT NULL::smallint, p_valid_from date DEFAULT NULL::date)
 RETURNS TABLE(rider_id uuid, status character varying, reason character varying, warnings jsonb)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_days   SMALLINT;
    v_from   DATE;
    v_pair   UUID;
    v_rider  UUID;
    v_home   BOOLEAN;
    v_legs   VARCHAR;
    v_ids    UUID[] := '{}';
    v_status VARCHAR[] := '{}';
    v_reason VARCHAR[] := '{}';
BEGIN
    IF cardinality(COALESCE(p_rider_ids, '{}')) > 100 THEN
        RAISE EXCEPTION 'assignment.bulk-too-many' USING ERRCODE = 'P0001';
    END IF;
    SELECT r.paired_route_id, COALESCE(p_days, (
        SELECT CAST(bit_or(s.days_of_week) AS SMALLINT) FROM tracking.route_schedules s
        WHERE s.route_id = r.id AND COALESCE(s.valid_until, CURRENT_DATE) >= CURRENT_DATE
    ), 31)
      INTO v_pair, v_days
    FROM tracking.routes r
    INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
    WHERE r.id = p_route_id AND tc.tenant_id = p_tenant_id;
    IF v_days IS NULL THEN
        RAISE EXCEPTION 'route.not-found' USING ERRCODE = 'P0001';
    END IF;
    v_from := COALESCE(p_valid_from, tracking.fn_route_today(p_route_id));

    FOREACH v_rider IN ARRAY COALESCE(p_rider_ids, '{}') LOOP
        SELECT rd.home_location IS NOT NULL OR rd.stop_place_id IS NOT NULL, rd.service_legs INTO v_home, v_legs
        FROM tracking.riders rd
        INNER JOIN tracking.organizations o ON o.id = rd.organization_id
        WHERE rd.id = v_rider AND o.tenant_id = p_tenant_id;

        v_ids := v_ids || v_rider;
        IF v_home IS NULL THEN
            v_status := v_status || CAST('skipped' AS VARCHAR);
            v_reason := v_reason || CAST('not-found' AS VARCHAR);
        ELSIF NOT v_home THEN
            v_status := v_status || CAST('skipped' AS VARCHAR);
            v_reason := v_reason || CAST('no-home' AS VARCHAR);
        ELSE
            BEGIN
                PERFORM 1 FROM tracking.sp_create_rider_route_assignments(p_tenant_id, jsonb_build_array(jsonb_build_object(
                    'rider_id', v_rider, 'route_id', p_route_id, 'days_of_week', v_days, 'valid_from', v_from,
                    'legs', CASE v_legs WHEN 'outbound' THEN 'outbound' WHEN 'return' THEN 'return' ELSE 'both' END)));
                v_status := v_status || CAST('assigned' AS VARCHAR);
                v_reason := v_reason || CAST(NULL AS VARCHAR);
            EXCEPTION WHEN raise_exception THEN
                v_status := v_status || CAST('skipped' AS VARCHAR);
                v_reason := v_reason || CAST(regexp_replace(SQLERRM, '^assignment\.', '') AS VARCHAR);
            END;
        END IF;
    END LOOP;

    RETURN QUERY
    SELECT u.rider_id, u.status, u.reason,
           tracking.fn_assignment_warnings(p_tenant_id, array_remove(ARRAY[p_route_id, v_pair], NULL))
    FROM unnest(v_ids, v_status, v_reason) WITH ORDINALITY AS u(rider_id, status, reason, ord)
    ORDER BY u.ord;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_bulk_assign_riders(uuid,uuid,uuid[],smallint,date) IS 'Assigns up to 100 riders to a route (and its return) in one call, each in its own subtransaction: answers per rider assigned or skipped with its reason (no-home, overlap, not-found…) and the routes'' capacity warnings (TRACK-039 D1); each rider rides the legs it contracted (TRACK-043 D1) and its shared stop counts as a home';
