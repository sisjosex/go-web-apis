CREATE OR REPLACE FUNCTION tracking.sp_get_driver(p_tenant_id uuid, p_driver_id uuid)
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
    WHERE d.id = p_driver_id AND d.tenant_id = p_tenant_id;
END;
$function$;
