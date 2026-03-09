-- Migration: sp_supplier_operations
-- Module: purchasing
-- Description: Stored procedures for supplier CRUD operations

-- CREATE SUPPLIER
CREATE OR REPLACE FUNCTION purchasing.sp_create_supplier(
    p_tenant_id UUID,
    p_name VARCHAR,
    p_contact_person VARCHAR,
    p_email VARCHAR,
    p_phone VARCHAR,
    p_address TEXT,
    p_payment_terms INT
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    name VARCHAR,
    contact_person VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    payment_terms INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    INSERT INTO purchasing.suppliers(tenant_id, name, contact_person, email, phone, address, payment_terms)
    VALUES(p_tenant_id, p_name, p_contact_person, p_email, p_phone, p_address, COALESCE(p_payment_terms, 30))
    RETURNING 
        suppliers.id,
        suppliers.tenant_id,
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
$$;

-- UPDATE SUPPLIER
CREATE OR REPLACE FUNCTION purchasing.sp_update_supplier(
    p_supplier_id UUID,
    p_tenant_id UUID,
    p_name VARCHAR,
    p_contact_person VARCHAR,
    p_email VARCHAR,
    p_phone VARCHAR,
    p_address TEXT,
    p_payment_terms INT,
    p_is_active BOOLEAN
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    name VARCHAR,
    contact_person VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    payment_terms INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    UPDATE purchasing.suppliers
    SET 
        name = COALESCE(p_name, name),
        contact_person = COALESCE(p_contact_person, contact_person),
        email = COALESCE(p_email, email),
        phone = COALESCE(p_phone, phone),
        address = COALESCE(p_address, address),
        payment_terms = COALESCE(p_payment_terms, payment_terms),
        is_active = COALESCE(p_is_active, is_active),
        updated_at = CURRENT_TIMESTAMP
    WHERE id = p_supplier_id AND tenant_id = p_tenant_id
    RETURNING 
        suppliers.id,
        suppliers.tenant_id,
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
$$;

-- DELETE SUPPLIER (soft delete)
CREATE OR REPLACE FUNCTION purchasing.sp_delete_supplier(
    p_supplier_id UUID,
    p_tenant_id UUID
)
RETURNS TABLE(
    deleted BOOLEAN
) LANGUAGE plpgsql AS $$
BEGIN
    UPDATE purchasing.suppliers 
    SET is_active = FALSE, updated_at = CURRENT_TIMESTAMP 
    WHERE id = p_supplier_id AND tenant_id = p_tenant_id;
    
    RETURN QUERY SELECT TRUE;
END;
$$;

-- GET SUPPLIERS COUNT
CREATE OR REPLACE FUNCTION purchasing.sp_get_suppliers_count(
    p_tenant_id UUID
)
RETURNS TABLE(
    total_count INT
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT CAST(COUNT(*) AS INT) FROM purchasing.suppliers 
    WHERE tenant_id = p_tenant_id AND is_active = TRUE;
END;
$$;

-- LIST SUPPLIERS (paginated)
CREATE OR REPLACE FUNCTION purchasing.sp_list_suppliers(
    p_tenant_id UUID,
    p_page_size INT,
    p_offset INT
)
RETURNS TABLE(
    id UUID,
    tenant_id UUID,
    name VARCHAR,
    contact_person VARCHAR,
    email VARCHAR,
    phone VARCHAR,
    address TEXT,
    payment_terms INT,
    is_active BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) LANGUAGE plpgsql AS $$
BEGIN
    RETURN QUERY
    SELECT 
        s.id,
        s.tenant_id,
        s.name,
        s.contact_person,
        s.email,
        s.phone,
        s.address,
        s.payment_terms,
        s.is_active,
        s.created_at,
        s.updated_at
    FROM purchasing.suppliers s
    WHERE s.tenant_id = p_tenant_id AND s.is_active = TRUE
    ORDER BY s.created_at DESC
    LIMIT p_page_size OFFSET p_offset;
END;
$$;
