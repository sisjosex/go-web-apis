CREATE OR REPLACE FUNCTION tracking.sp_get_company(p_company_id uuid)
 RETURNS TABLE(out_id uuid, out_name character varying, out_email character varying, out_phone character varying, out_address text, out_city character varying, out_country character varying, out_registration_number character varying, out_status character varying, out_created_at timestamp without time zone, out_updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    SELECT 
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
        updated_at
    FROM tracking.transport_companies
    WHERE id = p_company_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_get_company(p_tenant_id uuid, p_company_id uuid)
 RETURNS TABLE(id uuid, tenant_id uuid, name character varying, email character varying, phone character varying, address text, city character varying, country character varying, registration_number character varying, status character varying, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM tracking.transport_companies tc
        WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    ) THEN
        RAISE EXCEPTION 'company.not-found' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT
        tc.id,
        tc.tenant_id,
        CAST(tc.name AS VARCHAR),
        CAST(tc.email AS VARCHAR),
        CAST(tc.phone AS VARCHAR),
        tc.address,
        CAST(tc.city AS VARCHAR),
        CAST(tc.country AS VARCHAR),
        CAST(tc.registration_number AS VARCHAR),
        CAST(tc.status AS VARCHAR),
        tc.created_at,
        tc.updated_at
    FROM tracking.transport_companies tc
    WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id;
END;
$function$;
