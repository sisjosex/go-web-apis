CREATE OR REPLACE FUNCTION users.sp_user_invitation_target(p_tenant_id uuid, p_user_id uuid)
 RETURNS TABLE(email character varying, first_name character varying)
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM auth.users u
        INNER JOIN tenancy.tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = p_tenant_id AND tu.is_active = TRUE
        WHERE u.id = p_user_id AND u.deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'user.not-found' USING ERRCODE = 'P0001';
    END IF;

    IF EXISTS (SELECT 1 FROM auth.user_sessions us WHERE us.user_id = p_user_id) THEN
        RAISE EXCEPTION 'user.invitation.already-active' USING ERRCODE = 'P0001';
    END IF;

    RETURN QUERY
    SELECT CAST(u.email AS VARCHAR), CAST(u.first_name AS VARCHAR)
    FROM auth.users u
    WHERE u.id = p_user_id;
END;
$function$;

COMMENT ON FUNCTION users.sp_user_invitation_target(uuid,uuid) IS 'Who a resent invitation goes to: a person of this tenant who never signed in (USERS-015 D4). Raises user.not-found, or user.invitation.already-active once they did.';
