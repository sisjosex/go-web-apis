CREATE OR REPLACE FUNCTION tracking.sp_update_driver(p_tenant_id uuid, p_driver_id uuid, p_user_id uuid, p_clear_user_id boolean, p_first_name character varying, p_last_name character varying, p_phone character varying, p_license_number character varying, p_license_class character varying, p_license_expires_on date, p_status character varying)
 RETURNS TABLE(id uuid, tenant_id uuid, company_id uuid, company_name character varying, user_id uuid, user_email character varying, first_name character varying, last_name character varying, phone character varying, license_number character varying, license_class character varying, license_expires_on date, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'driver.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF p_license_number IS NOT NULL AND TRIM(p_license_number) <> '' AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id
          AND d.license_number = TRIM(p_license_number)
          AND d.id <> p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.license-already-exists' USING ERRCODE = 'P0001';
    END IF;

    IF p_user_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM tracking.drivers d
        WHERE d.tenant_id = p_tenant_id AND d.user_id = p_user_id AND d.id <> p_driver_id
    ) THEN
        RAISE EXCEPTION 'driver.user-already-linked' USING ERRCODE = 'P0001';
    END IF;

    UPDATE tracking.drivers d SET
        user_id            = CASE WHEN COALESCE(p_clear_user_id, false) THEN NULL
                                  ELSE COALESCE(p_user_id, d.user_id) END,
        first_name         = COALESCE(NULLIF(TRIM(p_first_name), ''), d.first_name),
        last_name          = COALESCE(NULLIF(TRIM(p_last_name), ''), d.last_name),
        phone              = COALESCE(NULLIF(TRIM(p_phone), ''), d.phone),
        license_number     = COALESCE(NULLIF(TRIM(p_license_number), ''), d.license_number),
        license_class      = COALESCE(NULLIF(TRIM(p_license_class), ''), d.license_class),
        license_expires_on = COALESCE(p_license_expires_on, d.license_expires_on),
        status             = COALESCE(NULLIF(TRIM(p_status), ''), d.status),
        updated_at         = CURRENT_TIMESTAMP
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id;

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
    WHERE d.id = p_driver_id;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_update_driver(uuid,uuid,uuid,boolean,character varying,character varying,character varying,character varying,character varying,date,character varying) IS 'Updates a driver; company_id is immutable, p_clear_user_id unlinks the account, and a licence or account already in use raises tracking.driver.license-already-exists / tracking.driver.user-already-linked';
