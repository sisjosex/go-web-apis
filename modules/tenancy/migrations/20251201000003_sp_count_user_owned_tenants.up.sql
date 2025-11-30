DROP FUNCTION IF EXISTS tenancy.sp_count_user_owned_tenants(UUID);

CREATE OR REPLACE FUNCTION tenancy.sp_count_user_owned_tenants(
    p_user_id UUID
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO v_count
    FROM tenancy.tenant_users
    WHERE user_id = p_user_id
      AND role = 'owner'
      AND is_active = TRUE;
    
    RETURN v_count;
END;
$$;

COMMENT ON FUNCTION tenancy.sp_count_user_owned_tenants(UUID) IS 'Counts the number of tenants where the user has the owner role. Used for subscription plan limits.';
