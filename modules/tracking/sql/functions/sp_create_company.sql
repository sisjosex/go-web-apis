CREATE OR REPLACE FUNCTION tracking.sp_create_company(p_name character varying, p_email character varying, p_phone character varying, p_address text, p_city character varying, p_country character varying, p_registration_number character varying, p_status character varying)
 RETURNS TABLE(out_id uuid, out_name character varying, out_email character varying, out_phone character varying, out_address text, out_city character varying, out_country character varying, out_registration_number character varying, out_status character varying, out_created_at timestamp without time zone, out_updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    -- Check if registration_number already exists
    IF EXISTS (
        SELECT 1 FROM tracking.transport_companies 
        WHERE transport_companies.registration_number = p_registration_number
    ) THEN
        RAISE EXCEPTION 'tracking.company.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.transport_companies (
        name, 
        email, 
        phone, 
        address, 
        city, 
        country, 
        registration_number, 
        status
    )
    VALUES (
        p_name, 
        NULLIF(p_email, ''), 
        NULLIF(p_phone, ''), 
        NULLIF(p_address, ''), 
        NULLIF(p_city, ''), 
        NULLIF(p_country, ''), 
        p_registration_number, 
        COALESCE(NULLIF(p_status, ''), 'active')
    )
    RETURNING 
        id,
        name,
        email,
        phone,
        address,
        city,
        country,
        registration_number,
        status,
        created_at,
        updated_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_create_company(p_tenant_id uuid, p_name character varying, p_email character varying, p_phone character varying, p_address text, p_city character varying, p_country character varying, p_registration_number character varying, p_status character varying)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, email character varying, phone character varying, address text, city character varying, country character varying, registration_number character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.registration_number = p_registration_number
          AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.registration-number.already-exists' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    INSERT INTO tracking.transport_companies (
        tenant_id, name, email, phone, address, city, country, registration_number, status
    )
    VALUES (
        p_tenant_id,
        TRIM(p_name),
        NULLIF(TRIM(p_email), ''),
        NULLIF(TRIM(p_phone), ''),
        NULLIF(TRIM(p_address), ''),
        NULLIF(TRIM(p_city), ''),
        NULLIF(TRIM(p_country), ''),
        TRIM(p_registration_number),
        COALESCE(NULLIF(TRIM(p_status), ''), 'active')
    )
    RETURNING
        tracking.transport_companies.id,
        tracking.transport_companies.tenant_id,
        CAST(tracking.transport_companies.name AS VARCHAR),
        CAST(tracking.transport_companies.email AS VARCHAR),
        CAST(tracking.transport_companies.phone AS VARCHAR),
        tracking.transport_companies.address,
        CAST(tracking.transport_companies.city AS VARCHAR),
        CAST(tracking.transport_companies.country AS VARCHAR),
        CAST(tracking.transport_companies.registration_number AS VARCHAR),
        CAST(tracking.transport_companies.status AS VARCHAR),
        tracking.transport_companies.created_at,
        tracking.transport_companies.updated_at;
END;
$function$;
