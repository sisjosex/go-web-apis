CREATE OR REPLACE FUNCTION purchasing.sp_create_supplier(p_tenant_id uuid, p_name character varying, p_contact_person character varying, p_email character varying, p_phone character varying, p_address text, p_payment_terms integer, p_tax_id character varying DEFAULT NULL::character varying, p_legal_name character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, tax_id character varying, legal_name character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.suppliers(tenant_id, name, contact_person, email, phone, address, payment_terms, tax_id, legal_name)
    VALUES(p_tenant_id, p_name, p_contact_person, p_email, p_phone, p_address, COALESCE(p_payment_terms, 30), p_tax_id, p_legal_name)
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
        suppliers.updated_at,
        suppliers.tax_id,
        suppliers.legal_name;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_create_supplier(uuid,character varying,character varying,character varying,character varying,text,integer,character varying,character varying) IS 'Creates a supplier and answers it whole; payment terms default to 30 days (PURCH-001).';
