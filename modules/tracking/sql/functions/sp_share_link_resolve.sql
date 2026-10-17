CREATE OR REPLACE FUNCTION tracking.sp_share_link_resolve(p_tenant_id uuid, p_token_hash bytea)
 RETURNS TABLE(trip_id uuid, rider_id uuid, expires_at timestamp with time zone)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT l.trip_id, l.rider_id, l.expires_at
    FROM tracking.share_links l
    WHERE l.token_hash = p_token_hash
      AND l.tenant_id = p_tenant_id
      AND l.revoked_at IS NULL
      AND l.expires_at > now();
$function$;

COMMENT ON FUNCTION tracking.sp_share_link_resolve(uuid,bytea) IS 'The trip and rider an open share link shows; no row for an unknown, expired or revoked token, which the caller answers as one 404 (TRACK-060)';
