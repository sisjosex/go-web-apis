CREATE OR REPLACE FUNCTION tracking.sp_update_company(p_company_id uuid, p_name character varying, p_email character varying, p_phone character varying, p_address text, p_city character varying, p_country character varying)
 RETURNS TABLE(out_id uuid, out_name character varying, out_email character varying, out_phone character varying, out_address text, out_city character varying, out_country character varying, out_registration_number character varying, out_status character varying, out_created_at timestamp without time zone, out_updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    UPDATE tracking.transport_companies t
    SET 
        name = COALESCE(p_name, t.name),
        email = COALESCE(p_email, t.email),
        phone = COALESCE(p_phone, t.phone),
        address = COALESCE(p_address, t.address),
        city = COALESCE(p_city, t.city),
        country = COALESCE(p_country, t.country),
        updated_at = CURRENT_TIMESTAMP
    WHERE t.id = p_company_id
    RETURNING 
        t.id,
        t.name,
        t.email,
        t.phone,
        t.address,
        t.city,
        t.country,
        t.registration_number,
        t.status,
        t.created_at,
        t.updated_at;
END;
$function$;

CREATE OR REPLACE FUNCTION tracking.sp_update_company(p_tenant_id uuid, p_company_id uuid, p_name character varying, p_email character varying, p_phone character varying, p_address text, p_city character varying, p_country character varying)
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
    UPDATE tracking.transport_companies tc SET
        name = COALESCE(NULLIF(TRIM(p_name), ''), tc.name),
        email = NULLIF(TRIM(p_email), ''),
        phone = NULLIF(TRIM(p_phone), ''),
        address = NULLIF(TRIM(p_address), ''),
        city = NULLIF(TRIM(p_city), ''),
        country = NULLIF(TRIM(p_country), ''),
        updated_at = CURRENT_TIMESTAMP
    WHERE tc.id = p_company_id AND tc.tenant_id = p_tenant_id
    RETURNING
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
        tc.updated_at;
END;
$function$;
