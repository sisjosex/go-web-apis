CREATE OR REPLACE FUNCTION tracking.sp_get_rider(p_rider_id uuid, p_guardian_user_id uuid)
 RETURNS TABLE(id uuid, company_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT
        tracking.riders.id,
        tracking.riders.company_id,
        NULL::VARCHAR(50),
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        (tracking.riders.emergency_contact->>'name')::VARCHAR(255),
        (tracking.riders.emergency_contact->>'phone')::VARCHAR(50),
        NULL::UUID,
        NULL::VARCHAR(255),
        NULL::VARCHAR(50),
        NULL::VARCHAR(255),
        NULL::VARCHAR(500),
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at
    FROM tracking.riders
    WHERE tracking.riders.id = p_rider_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'tracking.rider.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_get_rider(p_tenant_id uuid, p_rider_id uuid, p_guardian_user_id uuid, p_scope_user_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, organization_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, home_latitude numeric, home_longitude numeric, notes text, group_label character varying, service_legs character varying, stop_place_id uuid, stop_place_name character varying, organization_name character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
    v_riders UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
        v_riders := tracking.fn_supervised_riders(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
          AND (v_riders IS NULL OR r.id = ANY(v_riders))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        r.id,
        r.organization_id,
        CAST(r.rider_type AS VARCHAR),
        CAST(r.first_name AS VARCHAR),
        CAST(r.last_name AS VARCHAR),
        CAST(r.identification_number AS VARCHAR),
        CAST(r.phone AS VARCHAR),
        CAST(r.email AS VARCHAR),
        CAST(ec.name AS VARCHAR),
        CAST(ec.phone AS VARCHAR),
        gc.user_id,
        CAST(gc.name AS VARCHAR),
        CAST(gc.phone AS VARCHAR),
        CAST(gc.email AS VARCHAR),
        CAST(r.address AS VARCHAR),
        (r.status = 'active'),
        r.created_at,
        r.updated_at,
        CAST(ST_Y(r.home_location::geometry) AS DECIMAL),
        CAST(ST_X(r.home_location::geometry) AS DECIMAL),
        r.notes,
        CAST(r.group_label AS VARCHAR),
        CAST(r.service_legs AS VARCHAR),
        r.stop_place_id,
        CAST(sp.name AS VARCHAR),
        -- TRACK-077: the record page's client chip.
        CAST(org.name AS VARCHAR)
    FROM tracking.riders r
    INNER JOIN tracking.organizations org ON org.id = r.organization_id
    LEFT JOIN tracking.stop_places sp ON sp.id = r.stop_place_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_get_rider(uuid,uuid,uuid,uuid) IS 'One rider of the tenant with its primary guardian and emergency contacts, home, stop and client name (TRACK-077); an organization user (p_scope_user_id) reads only riders of its own organizations and groups, else rider.not-found';
