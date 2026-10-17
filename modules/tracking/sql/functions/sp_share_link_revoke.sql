CREATE OR REPLACE FUNCTION tracking.sp_share_link_revoke(p_tenant_id uuid, p_link_id uuid, p_user_id uuid, p_own_only boolean DEFAULT false)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    UPDATE tracking.share_links l
       SET revoked_at = now()
     WHERE l.id = p_link_id
       AND l.tenant_id = p_tenant_id
       AND l.revoked_at IS NULL
       AND (NOT p_own_only OR l.created_by = p_user_id);
    IF NOT FOUND THEN
        RAISE EXCEPTION 'share.not-found' USING ERRCODE = 'P0001';
    END IF;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_share_link_revoke(uuid,uuid,uuid,boolean) IS 'Closes a share link at once; p_own_only (a guardian) closes only links the user made; raises share.not-found (TRACK-060)';
