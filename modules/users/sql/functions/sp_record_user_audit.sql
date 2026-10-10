CREATE OR REPLACE FUNCTION users.sp_record_user_audit(p_tenant_id uuid, p_action character varying, p_target_user_id uuid DEFAULT NULL::uuid, p_performed_by uuid DEFAULT NULL::uuid, p_metadata jsonb DEFAULT NULL::jsonb, p_source character varying DEFAULT 'user'::character varying)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION users.sp_record_user_audit(uuid,character varying,uuid,uuid,jsonb,character varying) IS 'Records a user management audit event for a tenant';
