CREATE OR REPLACE FUNCTION tracking.fn_organization_row(p_tenant_id uuid, p_organization_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, empty_trip_cancel_min integer, delay_notice_min integer)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        o.id,
        o.tenant_id,
        CAST(o.kind AS VARCHAR),
        CAST(o.name AS VARCHAR),
        CAST(o.timezone AS VARCHAR),
        o.is_active,
        o.created_at,
        o.updated_at,
        o.absence_cutoff_min,
        CAST(ST_Y(o.location::geometry) AS DECIMAL),
        CAST(ST_X(o.location::geometry) AS DECIMAL),
        CAST(o.address AS VARCHAR),
        o.stop_place_id,
        o.empty_trip_cancel_min,
        o.delay_notice_min
    FROM tracking.organizations o
    WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id;
$function$;
