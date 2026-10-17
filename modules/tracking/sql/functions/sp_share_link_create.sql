CREATE OR REPLACE FUNCTION tracking.sp_share_link_create(p_tenant_id uuid, p_trip_id uuid, p_rider_id uuid, p_user_id uuid, p_via character varying, p_token_hash bytea, p_ttl_minutes integer DEFAULT NULL::integer)
 RETURNS TABLE(id uuid, trip_id uuid, expires_at timestamp with time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_trip UUID := p_trip_id;
    v_status VARCHAR;
    v_end TIMESTAMPTZ;
    v_expires TIMESTAMPTZ;
    v_id UUID;
BEGIN
    -- The guardian's link: its own rider, on the rider's trip of today still to run.
    IF p_rider_id IS NOT NULL THEN
        IF p_via = 'portal' AND NOT (p_rider_id = ANY(tracking.fn_guardian_scope(p_tenant_id, p_user_id))) THEN
            RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
        END IF;
        PERFORM 1
        FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id AND o.tenant_id = p_tenant_id;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
        END IF;

        SELECT t.id INTO v_trip
        FROM tracking.trips t
        INNER JOIN tracking.routes r ON r.id = t.route_id
        WHERE t.tenant_id = p_tenant_id
          AND t.status IN ('planned', 'in_progress')
          AND t.service_date = CAST(now() AT TIME ZONE r.timezone AS DATE)
          AND EXISTS (
                SELECT 1
                FROM tracking.trip_stops ts
                INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
                WHERE ts.trip_id = t.id
                  AND k.subject_type = 'passenger'
                  AND k.subject_id = p_rider_id
                  AND k.status <> 'cancelled')
        ORDER BY CASE t.status WHEN 'in_progress' THEN 0 ELSE 1 END, t.planned_start
        LIMIT 1;
        IF v_trip IS NULL THEN
            RAISE EXCEPTION 'share.no-trip' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    SELECT t.status,
           COALESCE((SELECT MAX(ts.planned_at) FROM tracking.trip_stops ts WHERE ts.trip_id = t.id), t.planned_start)
      INTO v_status, v_end
    FROM tracking.trips t
    WHERE t.id = v_trip AND t.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'trip.not-found' USING ERRCODE = 'P0001';
    END IF;
    IF v_status NOT IN ('planned', 'in_progress') THEN
        RAISE EXCEPTION 'share.no-trip' USING ERRCODE = 'P0001';
    END IF;

    -- Default: the trip's planned end + 60 min, never less than an hour from now; at most 24 h.
    v_expires := CASE
        WHEN p_ttl_minutes IS NOT NULL THEN now() + make_interval(mins => GREATEST(1, LEAST(p_ttl_minutes, 1440)))
        ELSE GREATEST(v_end + INTERVAL '60 minutes', now() + INTERVAL '60 minutes')
    END;
    v_expires := LEAST(v_expires, now() + INTERVAL '1440 minutes');

    INSERT INTO tracking.share_links (tenant_id, trip_id, rider_id, token_hash, created_by, created_via, expires_at)
    VALUES (p_tenant_id, v_trip, p_rider_id, p_token_hash, p_user_id, p_via, v_expires)
    RETURNING tracking.share_links.id INTO v_id;

    RETURN QUERY SELECT v_id, v_trip, v_expires;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_share_link_create(uuid,uuid,uuid,uuid,character varying,bytea,integer) IS 'Stores a share link''s token hash for a planned or running trip (p_rider_id NULL) or a rider''s trip of today (a guardian: own riders only); ttl clamped to 24 h, NULL = the trip''s planned end + 60 min; raises trip.not-found, rider.not-found, share.no-trip (TRACK-060)';
