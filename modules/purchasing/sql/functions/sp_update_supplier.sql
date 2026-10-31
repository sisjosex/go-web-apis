CREATE OR REPLACE FUNCTION purchasing.sp_update_supplier(p_tenant_id uuid, p_supplier_id uuid, p_name character varying, p_contact_person character varying, p_email character varying, p_phone character varying, p_address text, p_payment_terms integer, p_is_active boolean, p_tax_id character varying DEFAULT NULL::character varying, p_legal_name character varying DEFAULT NULL::character varying)
 RETURNS TABLE(id uuid, name character varying, contact_person character varying, email character varying, phone character varying, address text, payment_terms integer, is_active boolean, created_at timestamp without time zone, updated_at timestamp without time zone, tax_id character varying, legal_name character varying)
 LANGUAGE plpgsql
AS $function$
BEGIN
    RETURN QUERY
    UPDATE purchasing.suppliers s
    SET
        name = COALESCE(p_name, s.name),
        contact_person = COALESCE(p_contact_person, s.contact_person),
        email = COALESCE(p_email, s.email),
        phone = COALESCE(p_phone, s.phone),
        address = COALESCE(p_address, s.address),
        payment_terms = COALESCE(p_payment_terms, s.payment_terms),
        is_active = COALESCE(p_is_active, s.is_active),
        tax_id = COALESCE(p_tax_id, s.tax_id),
        legal_name = COALESCE(p_legal_name, s.legal_name),
        updated_at = CURRENT_TIMESTAMP
    WHERE s.id = p_supplier_id AND s.tenant_id = p_tenant_id
    RETURNING
        s.id,
        s.name,
        s.contact_person,
        s.email,
        s.phone,
        s.address,
        s.payment_terms,
        s.is_active,
        s.created_at,
        s.updated_at,
        s.tax_id,
        s.legal_name;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'supplier.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION purchasing.sp_update_supplier(uuid,uuid,character varying,character varying,character varying,character varying,text,integer,boolean,character varying,character varying) IS 'Changes the given fields of a supplier, a NULL leaves one as it is; is_active reactivates. Raises supplier.not-found.';
