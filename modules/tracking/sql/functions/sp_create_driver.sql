CREATE OR REPLACE FUNCTION tracking.sp_create_driver(p_tenant_id uuid, p_company_id uuid, p_user_id uuid, p_first_name character varying, p_last_name character varying, p_phone character varying, p_license_number character varying, p_license_class character varying, p_license_expires_on date, p_status character varying)
 RETURNS TABLE(id uuid, tenant_id uuid, company_id uuid, company_name character varying, user_id uuid, user_email character varying, first_name character varying, last_name character varying, phone character varying, license_number character varying, license_class character varying, license_expires_on date, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_driver_id UUID;
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.license_number = NULLIF(TRIM(p_license_number), '')
    ) THEN
        RAISE EXCEPTION 'driver.license-already-exists' USING ERRCODE = 'P0001';
    END IF;

    IF p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id
    ) THEN
        RAISE EXCEPTION 'driver.user-already-linked' USING ERRCODE = 'P0001';
    END IF;

    INSERT INTO tracking.drivers (
        tenant_id, company_id, user_id, first_name, last_name, phone,
        license_number, license_class, license_expires_on, status
    )
    VALUES (
        p_tenant_id,
        p_company_id,
        p_user_id,
        TRIM(p_first_name),
        TRIM(p_last_name),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_license_number), ''),
        NULLIF(TRIM(p_license_class), ''),
        p_license_expires_on,
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING tracking.drivers.id INTO v_driver_id;

    RETURN QUERY
    SELECT
        d.id,
        d.tenant_id,
        d.company_id,
        CAST(tc.name AS VARCHAR),
        d.user_id,
        CAST(u.email AS VARCHAR),
        CAST(d.first_name AS VARCHAR),
        CAST(d.last_name AS VARCHAR),
        CAST(d.phone AS VARCHAR),
        CAST(d.license_number AS VARCHAR),
        CAST(d.license_class AS VARCHAR),
        d.license_expires_on,
        CAST(d.status AS VARCHAR),
        d.created_at,
        d.updated_at
    FROM tracking.drivers d
    INNER JOIN tracking.transport_companies tc ON tc.id = d.company_id
    LEFT JOIN auth.users u ON u.id = d.user_id AND u.deleted_at IS NULL
    WHERE d.id = v_driver_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_create_driver(uuid,uuid,uuid,character varying,character varying,character varying,character varying,character varying,date,character varying) IS 'Creates a driver for one of the tenant''s carriers, the licence optional (TRACK-047); raises tracking.company.not-found, tracking.driver.license-already-exists or tracking.driver.user-already-linked';
