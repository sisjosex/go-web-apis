CREATE OR REPLACE FUNCTION tracking.sp_update_organization(p_tenant_id uuid, p_organization_id uuid, p_kind character varying, p_name character varying, p_timezone character varying, p_is_active boolean, p_absence_cutoff_min integer DEFAULT NULL::integer, p_latitude numeric DEFAULT NULL::numeric, p_longitude numeric DEFAULT NULL::numeric, p_address character varying DEFAULT NULL::character varying, p_create_stop boolean DEFAULT true, p_empty_trip_cancel_min integer DEFAULT NULL::integer)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, empty_trip_cancel_min integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE tracking.organizations o SET
        kind      = COALESCE(NULLIF(TRIM(p_kind), ''), o.kind),
        name      = COALESCE(NULLIF(TRIM(p_name), ''), o.name),
        timezone  = COALESCE(NULLIF(TRIM(p_timezone), ''), o.timezone),
        is_active = COALESCE(p_is_active, o.is_active),
        absence_cutoff_min = COALESCE(p_absence_cutoff_min, o.absence_cutoff_min),
        location  = CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography ELSE o.location END,
        address   = COALESCE(NULLIF(TRIM(p_address), ''), o.address),
        empty_trip_cancel_min = COALESCE(p_empty_trip_cancel_min, o.empty_trip_cancel_min),
        updated_at = CURRENT_TIMESTAMP
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    PERFORM tracking.fn_organization_stop_sync(p_tenant_id, p_organization_id, p_create_stop);
    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, p_organization_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_organization(uuid,uuid,character varying,character varying,character varying,boolean,integer,numeric,numeric,character varying,boolean,integer) IS 'Edits a client; its stop moves and renames with it, and a pinned client without one gets one unless p_create_stop is false (TRACK-045 D3)';
