CREATE OR REPLACE FUNCTION tracking.sp_create_rider(p_company_id uuid, p_rider_type character varying, p_first_name character varying, p_last_name character varying, p_identification_number character varying, p_phone character varying, p_email character varying, p_emergency_contact_name character varying, p_emergency_contact_phone character varying, p_guardian_user_id uuid, p_guardian_name character varying, p_guardian_phone character varying, p_guardian_email character varying, p_address character varying)
 RETURNS TABLE(id uuid, company_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if company exists
    IF NOT EXISTS (SELECT 1 FROM tracking.transport_companies WHERE tracking.transport_companies.id = p_company_id) THEN
        RAISE EXCEPTION 'tracking.company.not-found' USING ERRCODE = 'P0001';
    END IF;

    -- Validate rider_type enum
    IF p_rider_type NOT IN ('student', 'employee') THEN
        RAISE EXCEPTION 'tracking.rider.invalid-type' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.riders (
        company_id, first_name, last_name, email, phone, identification_number, emergency_contact, status
    )
    VALUES (
        p_company_id, 
        p_first_name, 
        p_last_name, 
        p_email, 
        p_phone, 
        p_identification_number,
        -- Store emergency contact as JSON (existing column)
        CASE 
            WHEN p_emergency_contact_name IS NOT NULL OR p_emergency_contact_phone IS NOT NULL THEN
                jsonb_build_object(
                    'name', p_emergency_contact_name,
                    'phone', p_emergency_contact_phone
                )
            ELSE NULL
        END,
        'active'
    )
    RETURNING
        tracking.riders.id,
        tracking.riders.company_id,
        p_rider_type,                           -- Pass through (not in table)
        tracking.riders.first_name,
        tracking.riders.last_name,
        tracking.riders.identification_number,
        tracking.riders.phone,
        tracking.riders.email,
        p_emergency_contact_name,               -- Pass through (extracted from JSON)
        p_emergency_contact_phone,              -- Pass through (extracted from JSON)
        p_guardian_user_id,                     -- Pass through (not in table)
        p_guardian_name,                        -- Pass through (not in table)
        p_guardian_phone,                       -- Pass through (not in table)
        p_guardian_email,                       -- Pass through (not in table)
        p_address,                              -- Pass through (not in table)
        (tracking.riders.status = 'active'),
        tracking.riders.created_at,
        tracking.riders.updated_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_create_rider(p_tenant_id uuid, p_organization_id uuid, p_rider_type character varying, p_first_name character varying, p_last_name character varying, p_identification_number character varying, p_phone character varying, p_email character varying, p_emergency_contact_name character varying, p_emergency_contact_phone character varying, p_guardian_user_id uuid, p_guardian_name character varying, p_guardian_phone character varying, p_guardian_email character varying, p_address character varying, p_scope_user_id uuid DEFAULT NULL::uuid, p_home_latitude numeric DEFAULT NULL::numeric, p_home_longitude numeric DEFAULT NULL::numeric, p_notes text DEFAULT NULL::text, p_group_label character varying DEFAULT NULL::character varying, p_service_legs character varying DEFAULT NULL::character varying, p_stop_place_id uuid DEFAULT NULL::uuid)
 RETURNS TABLE(id uuid, organization_id uuid, rider_type character varying, first_name character varying, last_name character varying, identification_number character varying, phone character varying, email character varying, emergency_contact_name character varying, emergency_contact_phone character varying, guardian_user_id uuid, guardian_name character varying, guardian_phone character varying, guardian_email character varying, address character varying, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, home_latitude numeric, home_longitude numeric, notes text, group_label character varying, service_legs character varying, stop_place_id uuid, stop_place_name character varying)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_rider_id UUID;
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM tracking.organizations o
        WHERE o.id = p_organization_id
          AND o.tenant_id = p_tenant_id
          AND (v_scope IS NULL OR o.id = ANY(v_scope))
    ) THEN
        RAISE EXCEPTION 'organization.not-found' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.riders (
        organization_id, rider_type, first_name, last_name,
        identification_number, phone, email, status, home_location, notes, group_label,
        address, service_legs, stop_place_id
    )
    VALUES (
        p_organization_id,
        COALESCE(NULLIF(TRIM(p_rider_type), ''), 'employee'),
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_identification_number), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_email), ''),
        'active',
        CASE WHEN p_home_latitude IS NOT NULL AND p_home_longitude IS NOT NULL
            THEN ST_SetSRID(ST_MakePoint(p_home_longitude, p_home_latitude), 4326)::geography END,
        NULLIF(TRIM(p_notes), ''),
        NULLIF(TRIM(p_group_label), ''),
        NULLIF(TRIM(p_address), ''),
        COALESCE(NULLIF(TRIM(p_service_legs), ''), 'both'),
        tracking.fn_rider_stop_place(p_tenant_id, p_stop_place_id)
    )
    RETURNING tracking.riders.id INTO v_rider_id;

    PERFORM tracking.fn_rider_home_place(v_rider_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'guardian', p_guardian_name, p_guardian_phone, p_guardian_email, p_guardian_user_id);
    PERFORM tracking.fn_upsert_primary_rider_contact(
        v_rider_id, 'emergency', p_emergency_contact_name, p_emergency_contact_phone, NULL, NULL);

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
    WHERE r.id = v_rider_id;
END;
$function$;
