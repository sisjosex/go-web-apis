CREATE OR REPLACE FUNCTION tenancy.sp_accept_app_invites(p_user_id uuid)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count INT;
BEGIN
    UPDATE tenancy.app_invites i
    SET accepted_at = now()
    WHERE i.user_id = p_user_id AND i.accepted_at IS NULL;
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_accept_app_invites(uuid) IS 'Marks every open invitation of the account accepted, called when it signs in; answers how many (TRACK-062 D3)';
