CREATE OR REPLACE FUNCTION tenancy.sp_mark_app_invite_sent(p_tenant_id uuid, p_user_id uuid)
 RETURNS timestamp with time zone
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sent TIMESTAMPTZ;
BEGIN
    -- A guardian granted before invitations were kept has none: the first WhatsApp send starts it.
    INSERT INTO tenancy.app_invites (tenant_id, user_id, level, channel, sent_at)
    SELECT p_tenant_id, p_user_id, 'portal', 'whatsapp', now()
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = p_tenant_id AND tu.user_id = p_user_id AND tu.role = 'portal' AND tu.is_active
    ON CONFLICT ON CONSTRAINT uq_app_invites_tenant_user DO UPDATE SET sent_at = now()
    RETURNING sent_at INTO v_sent;
    IF v_sent IS NULL THEN
        RAISE EXCEPTION 'invite.not-found' USING ERRCODE = 'P0001';
    END IF;
    RETURN v_sent;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_mark_app_invite_sent(uuid,uuid) IS 'Records that an invitation went out now: the operator sent it by WhatsApp, or the email went again; an active portal account without one gets a WhatsApp invitation; raises invite.not-found (TRACK-062)';
