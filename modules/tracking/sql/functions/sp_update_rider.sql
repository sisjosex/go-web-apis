CREATE OR REPLACE FUNCTION tracking.sp_update_rider(p_rider_id uuid, p_phone character varying, p_email character varying, p_emergency_contact_name character varying, p_emergency_contact_phone character varying, p_guardian_user_id uuid, p_guardian_name character varying, p_guardian_phone character varying, p_guardian_email character varying, p_address character varying, p_is_active boolean)
 RETURNS TABLE(id uuid, company_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_current_emergency_contact JSONB;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tracking.riders WHERE tracking.riders.id = p_rider_id) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    SELECT tracking.riders.emergency_contact INTO v_current_emergency_contact
    FROM tracking.riders 
    WHERE tracking.riders.id = p_rider_id;

    RETURN QUERY
    UPDATE tracking.riders
    SET
        email = COALESCE(p_email, tracking.riders.email),
        phone = COALESCE(p_phone, tracking.riders.phone),
        emergency_contact = CASE 
            WHEN p_emergency_contact_name IS NOT NULL OR p_emergency_contact_phone IS NOT NULL THEN
                jsonb_build_object(
                    'name', COALESCE(p_emergency_contact_name, v_current_emergency_contact->>'name'),
                    'phone', COALESCE(p_emergency_contact_phone, v_current_emergency_contact->>'phone')
                )
            ELSE tracking.riders.emergency_contact
        END,
        status = CASE 
            WHEN p_is_active IS NOT NULL THEN 
                CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END 
            ELSE tracking.riders.status 
        END,
        updated_at = CURRENT_TIMESTAMP
    WHERE tracking.riders.id = p_rider_id
    RETURNING
        tracking.riders.id,
        tracking.riders.company_id,
        NULL::VARCHAR(50),
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        COALESCE(p_emergency_contact_name, (tracking.riders.emergency_contact->>'name')::VARCHAR(255)),
        COALESCE(p_emergency_contact_phone, (tracking.riders.emergency_contact->>'phone')::VARCHAR(50)),
        p_guardian_user_id,
        p_guardian_name,
        p_guardian_phone,
        p_guardian_email,
        p_address,
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_update_rider(p_tenant_id uuid, p_rider_id uuid, p_phone character varying, p_email character varying, p_emergency_contact_name character varying, p_emergency_contact_phone character varying, p_guardian_user_id uuid, p_guardian_name character varying, p_guardian_phone character varying, p_guardian_email character varying, p_address character varying, p_is_active boolean, p_scope_user_id uuid DEFAULT NULL::uuid, p_home_latitude numeric DEFAULT NULL::numeric, p_home_longitude numeric DEFAULT NULL::numeric, p_notes text DEFAULT NULL::text, p_group_label character varying DEFAULT NULL::character varying, p_service_legs character varying DEFAULT NULL::character varying, p_stop_place_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, organization_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, home_latitude numeric, home_longitude numeric, notes text, group_label character varying, service_legs character varying, stop_place_id uuid, stop_place_name character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE r.id = p_rider_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR r.organization_id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.riders r SET
        phone = NULLIF(TRIM(p_phone), ''),
        email = NULLIF(TRIM(p_email), ''),
        status = CASE
            WHEN p_is_active IS NOT NULL THEN CASE WHEN p_is_active THEN 'active' ELSE 'inactive' END
            ELSE r.status
        END,
        -- The contact retention counts from the day the rider stopped riding (TRACK-063 D3).
        deactivated_at = CASE
            WHEN p_is_active IS NULL THEN r.deactivated_at
            WHEN p_is_active THEN NULL
            ELSE COALESCE(r.deactivated_at, now())
        END,
        home_location = CASE WHEN p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_home_longitude, p_home_latitude), 4326)::geography
            ELSE r.home_location END,
        -- A person placing the pin confirms it (TRACK-072 D3).
        home_geocoded = r.home_geocoded AND (p_home_latitude IS NULL OR p_home_longitude IS NULL),
        notes = NULLIF(TRIM(p_notes), ''),
        -- NULL keeps the group, '' clears it (TRACK-039 D2).
        group_label = CASE WHEN p_group_label IS NULL THEN r.group_label ELSE NULLIF(TRIM(p_group_label), '') END,
        -- TRACK-043: NULL keeps each of these; '' clears the address.
        address = CASE WHEN p_address IS NULL THEN r.address ELSE NULLIF(TRIM(p_address), '') END,
        service_legs = COALESCE(NULLIF(TRIM(p_service_legs), ''), r.service_legs),
        stop_place_id = COALESCE(tracking.fn_rider_stop_place(p_tenant_id, p_stop_place_id), r.stop_place_id),
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = p_rider_id;

    IF p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL THEN
        PERFORM tracking.fn_rider_home_place(p_rider_id);
    END IF;
    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        p_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

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
        CAST(sp.name AS VARCHAR)
    FROM tracking.riders r
    LEFT JOIN tracking.stop_places sp ON sp.id = r.stop_place_id
    LEFT JOIN tracking.rider_contacts gc ON gc.rider_id = r.id AND gc.relation = 'guardian' AND gc.is_primary
    LEFT JOIN tracking.rider_contacts ec ON ec.rider_id = r.id AND ec.relation = 'emergency' AND ec.is_primary
    WHERE r.id = p_rider_id;
END;
$function$;
