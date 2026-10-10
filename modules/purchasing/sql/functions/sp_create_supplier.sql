CREATE OR REPLACE FUNCTION purchasing.sp_create_supplier(p_tenant_id uuid, p_name character varying, p_contact_person character varying, p_email character varying, p_phone character varying, p_address text, p_payment_terms integer)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.suppliers(tenant_id, name, contact_person, email, phone, address, payment_terms)
    VALUES(p_tenant_id, p_name, p_contact_person, p_email, p_phone, p_address, COALESCE(p_payment_terms, 30))
    RETURNING
        suppliers.id,
        suppliers.name,
        suppliers.contact_person,
        suppliers.email,
        suppliers.phone,
        suppliers.address,
        suppliers.payment_terms,
        suppliers.is_active,
        suppliers.created_at,
        suppliers.updated_at;
END;
$function$;
