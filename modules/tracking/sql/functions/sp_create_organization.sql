CREATE OR REPLACE FUNCTION tracking.sp_create_organization(p_tenant_id uuid, p_kind character varying, p_name character varying, p_timezone character varying, p_is_active boolean, p_absence_cutoff_min integer DEFAULT NULL::integer, p_latitude numeric DEFAULT NULL::numeric, p_longitude numeric DEFAULT NULL::numeric, p_address character varying DEFAULT NULL::character varying, p_create_stop boolean DEFAULT true)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id UUID;
BEGIN
    INSERT INTO tracking.organizations (tenant_id, kind, name, timezone, is_active, absence_cutoff_min, location, address)
    VALUES (
        p_tenant_id,
        COALESCE(NULLIF(TRIM(p_kind), ''), 'other'),
        TRIM(p_name),
        COALESCE(NULLIF(TRIM(p_timezone), ''), 'UTC'),
        COALESCE(p_is_active, true),
        COALESCE(p_absence_cutoff_min, 60),
        CASE WHEN p_latitude IS NOT NULL AND p_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_longitude, p_latitude), 4326)::geography END,
        NULLIF(TRIM(p_address), '')
    )
    RETURNING tracking.organizations.id INTO v_id;

    PERFORM tracking.fn_organization_stop_sync(p_tenant_id, v_id, p_create_stop);
    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, v_id);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_organization(uuid,character varying,character varying,character varying,boolean,integer,numeric,numeric,character varying,boolean) IS 'Creates a client; with a pin and p_create_stop (default) it also gets its own stop (TRACK-045 D3)';
