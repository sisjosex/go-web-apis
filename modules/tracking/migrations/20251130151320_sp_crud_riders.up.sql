-- Migration: sp_crud_riders
-- Module: tracking
-- Created: 2025-11-30 15:13:20

-- Riders CRUD with guardian access
CREATE OR REPLACE FUNCTION tracking.sp_create_rider(p_company_id UUID, p_rider_type VARCHAR(50), p_first_name VARCHAR(255), p_last_name VARCHAR(255), p_guardian_user_id UUID DEFAULT NULL, p_phone VARCHAR(50) DEFAULT NULL, p_email VARCHAR(255) DEFAULT NULL, p_address TEXT DEFAULT NULL)
RETURNS TABLE(id UUID, company_id UUID, rider_type VARCHAR(50), first_name VARCHAR(255), last_name VARCHAR(255), guardian_user_id UUID, phone VARCHAR(50), email VARCHAR(255), address TEXT, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    IF p_rider_type NOT IN ('student', 'employee') THEN RAISE EXCEPTION 'invalid_rider_type'; END IF;
    RETURN QUERY INSERT INTO tracking.riders (company_id, rider_type, first_name, last_name, guardian_user_id, phone, email, address)
    VALUES (p_company_id, p_rider_type, p_first_name, p_last_name, p_guardian_user_id, p_phone, p_email, p_address)
    RETURNING riders.id, riders.company_id, riders.rider_type, riders.first_name, riders.last_name, riders.guardian_user_id, riders.phone, riders.email, riders.address, riders.is_active, riders.created_at, riders.updated_at;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_update_rider(p_rider_id UUID, p_phone VARCHAR(50) DEFAULT NULL, p_email VARCHAR(255) DEFAULT NULL, p_address TEXT DEFAULT NULL, p_is_active BOOLEAN DEFAULT NULL)
RETURNS TABLE(id UUID, company_id UUID, rider_type VARCHAR(50), first_name VARCHAR(255), last_name VARCHAR(255), guardian_user_id UUID, phone VARCHAR(50), email VARCHAR(255), address TEXT, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY UPDATE tracking.riders SET phone = COALESCE(p_phone, riders.phone), email = COALESCE(p_email, riders.email), address = COALESCE(p_address, riders.address), is_active = COALESCE(p_is_active, riders.is_active), updated_at = CURRENT_TIMESTAMP WHERE riders.id = p_rider_id
    RETURNING riders.id, riders.company_id, riders.rider_type, riders.first_name, riders.last_name, riders.guardian_user_id, riders.phone, riders.email, riders.address, riders.is_active, riders.created_at, riders.updated_at;
    IF NOT FOUND THEN RAISE EXCEPTION 'rider_not_found'; END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_list_riders(p_company_id UUID DEFAULT NULL, p_guardian_user_id UUID DEFAULT NULL, p_rider_type VARCHAR(50) DEFAULT NULL, p_is_active BOOLEAN DEFAULT NULL)
RETURNS TABLE(id UUID, company_id UUID, rider_type VARCHAR(50), first_name VARCHAR(255), last_name VARCHAR(255), guardian_user_id UUID, phone VARCHAR(50), email VARCHAR(255), address TEXT, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY SELECT r.id, r.company_id, r.rider_type, r.first_name, r.last_name, r.guardian_user_id, r.phone, r.email, r.address, r.is_active, r.created_at, r.updated_at
    FROM tracking.riders r WHERE (p_company_id IS NULL OR r.company_id = p_company_id) AND (p_guardian_user_id IS NULL OR r.guardian_user_id = p_guardian_user_id) AND (p_rider_type IS NULL OR r.rider_type = p_rider_type) AND (p_is_active IS NULL OR r.is_active = p_is_active) ORDER BY r.created_at DESC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_get_rider(p_rider_id UUID, p_guardian_user_id UUID DEFAULT NULL)
RETURNS TABLE(id UUID, company_id UUID, rider_type VARCHAR(50), first_name VARCHAR(255), last_name VARCHAR(255), guardian_user_id UUID, phone VARCHAR(50), email VARCHAR(255), address TEXT, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP) AS $$
BEGIN
    RETURN QUERY SELECT r.id, r.company_id, r.rider_type, r.first_name, r.last_name, r.guardian_user_id, r.phone, r.email, r.address, r.is_active, r.created_at, r.updated_at
    FROM tracking.riders r WHERE r.id = p_rider_id AND (p_guardian_user_id IS NULL OR r.guardian_user_id = p_guardian_user_id);
    IF NOT FOUND THEN RAISE EXCEPTION 'rider_not_found_or_access_denied'; END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION tracking.sp_delete_rider(p_rider_id UUID) RETURNS BOOLEAN AS $$
BEGIN
    UPDATE tracking.riders SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = p_rider_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'rider_not_found'; END IF;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;
-- Example:
-- CREATE TABLE IF NOT EXISTS auth.my_table (
--     id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

