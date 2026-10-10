CREATE OR REPLACE FUNCTION tracking.fn_document_subject_name(p_tenant_id uuid, p_subject_type character varying, p_subject_id uuid)
 RETURNS character varying
 LANGUAGE sql
 STABLE
AS $function$
    SELECT CASE p_subject_type
        WHEN 'driver' THEN (
            SELECT CAST(d.first_name || ' ' || d.last_name AS VARCHAR)
            FROM tracking.drivers d
            WHERE d.id = p_subject_id AND d.tenant_id = p_tenant_id
        )
        WHEN 'vehicle' THEN (
            SELECT CAST(v.plate_number AS VARCHAR)
            FROM tracking.vehicles v
            INNER JOIN tracking.transport_companies tc ON tc.id = v.company_id
            WHERE v.id = p_subject_id AND tc.tenant_id = p_tenant_id
        )
    END;
$function$;
