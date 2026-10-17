CREATE OR REPLACE FUNCTION tracking.sp_get_organization(p_tenant_id uuid, p_organization_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, kind character varying, name character varying, timezone character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, absence_cutoff_min integer, latitude numeric, longitude numeric, address character varying, stop_place_id uuid, empty_trip_cancel_min integer, delay_notice_min integer)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id AND o.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY SELECT * FROM tracking.fn_organization_row(p_tenant_id, p_organization_id);
END;
$function$;
