CREATE OR REPLACE FUNCTION tracking.sp_list_rider_cards(p_tenant_id uuid, p_route_id uuid DEFAULT NULL::uuid, p_organization_id uuid DEFAULT NULL::uuid, p_rider_id uuid DEFAULT NULL::uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(rider_id uuid, first_name character varying, last_name character varying, organization_name character varying, route_name character varying, pickup_stop_name character varying, qr_token text)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
    v_pair UUID;
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF p_route_id IS NOT NULL THEN
        SELECT r.paired_route_id INTO v_pair FROM tracking.routes r WHERE r.id = p_route_id;
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(o.name AS VARCHAR),
        asg.route_name,
        asg.stop_name,
        r.qr_token
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    -- The route the card names: the outbound of the rider's current assignments (of p_route_id's pair
    -- when set), with where it boards and that stop's place in today's version for the order.
    LEFT JOIN LATERAL (
        SELECT
            CAST(ar.route_name AS VARCHAR) AS route_name,
            CAST(sp.name AS VARCHAR) AS stop_name,
            (
                SELECT MIN(vs.sequence)
                FROM tracking.route_versions rv
                INNER JOIN tracking.route_version_stops vs ON vs.version_id = rv.id
                WHERE rv.route_id = ar.id
                  AND vs.stop_place_id = a.pickup_stop_place_id
                  AND rv.effective_from <= CURRENT_DATE
                  AND (rv.effective_to IS NULL OR rv.effective_to >= CURRENT_DATE)
            ) AS seq
        FROM tracking.rider_route_assignments a
        INNER JOIN tracking.routes ar ON ar.id = a.route_id
        LEFT JOIN tracking.stop_places sp ON sp.id = a.pickup_stop_place_id
        WHERE a.rider_id = r.id
          AND COALESCE(a.valid_until, CURRENT_DATE) >= CURRENT_DATE
          AND (p_route_id IS NULL OR a.route_id IN (p_route_id, v_pair))
        ORDER BY (ar.direction = 'outbound') DESC, a.valid_from
        LIMIT 1
    ) asg ON true
    WHERE o.tenant_id = p_tenant_id
      AND (v_scope IS NULL OR o.id = ANY(v_scope))
      AND (p_organization_id IS NULL OR r.organization_id = p_organization_id)
      AND (p_route_id IS NULL OR asg.route_name IS NOT NULL)
      -- One rider's card prints whatever its status; a sheet carries only the active ones.
      AND (CASE WHEN p_rider_id IS NULL THEN r.status = 'active' ELSE r.id = p_rider_id END)
    ORDER BY asg.seq ASC NULLS LAST, r.last_name ASC, r.first_name ASC, r.id ASC;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_cards(uuid,uuid,uuid,uuid,uuid) IS 'The printable cards of a route''s riders (the pair''s too, in boarding order), of an organization or of one rider: name, organization, the outbound route and pickup stop, and the token a driver scans; narrowed to p_scope_user_id''s organizations (TRACK-072 D2)';
