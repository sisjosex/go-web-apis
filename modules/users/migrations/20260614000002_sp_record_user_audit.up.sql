CREATE FUNCTION users.sp_record_user_audit(
    p_tenant_id      UUID,
    p_action         VARCHAR(30),
    p_target_user_id UUID        DEFAULT NULL,
    p_performed_by   UUID        DEFAULT NULL,
    p_metadata       JSONB       DEFAULT NULL,
    p_source         VARCHAR(10) DEFAULT 'user'
) RETURNS VOID AS $$
BEGIN
    INSERT INTO users.user_audit_log (
        tenant_id,
        action,
        target_user_id,
        performed_by,
        metadata,
        source
    ) VALUES (
        p_tenant_id,
        p_action,
        p_target_user_id,
        p_performed_by,
        p_metadata,
        p_source
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION users.sp_record_user_audit IS 'Records a user management audit event for a tenant';
