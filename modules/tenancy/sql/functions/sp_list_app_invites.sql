CREATE OR REPLACE FUNCTION tenancy.sp_list_app_invites(p_tenant_id uuid, p_user_ids uuid[] DEFAULT NULL::uuid[], p_status character varying DEFAULT NULL::character varying)
 RETURNS TABLE(user_id uuid, channel character varying, destination character varying, sent_at timestamp with time zone, accepted_at timestamp with time zone, status character varying)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT s.user_id, s.channel, s.destination, s.sent_at, s.accepted_at, s.status
    FROM (
        SELECT i.user_id, CAST(i.channel AS VARCHAR) AS channel, CAST(i.destination AS VARCHAR) AS destination,
               i.sent_at, i.accepted_at,
               -- Derived, never stored (TRACK-062): accepted at the first sign-in, no response a month
               -- after it went out.
               CAST(CASE
                   WHEN i.accepted_at IS NOT NULL THEN 'accepted'
                   WHEN i.sent_at IS NULL THEN 'pending'
                   WHEN i.sent_at < now() - INTERVAL '30 days' THEN 'expired'
                   ELSE 'sent'
               END AS VARCHAR) AS status
        FROM tenancy.app_invites i
        WHERE i.tenant_id = p_tenant_id
          AND (p_user_ids IS NULL OR i.user_id = ANY(p_user_ids))
    ) s
    WHERE p_status IS NULL OR s.status = p_status;
$function$;

COMMENT ON FUNCTION tenancy.sp_list_app_invites(uuid,uuid[],character varying) IS 'The invitations of one business, of the given accounts and/or in one status (pending, sent, accepted, expired: sent over 30 days ago without a sign-in), one read per list (TRACK-062 D3)';
