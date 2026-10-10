CREATE OR REPLACE FUNCTION tracking.sp_list_notifications(p_tenant_id uuid, p_user_id uuid, p_unread boolean DEFAULT false, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS jsonb
 LANGUAGE sql
 STABLE
AS $function$
    WITH mine AS (
        SELECT n.*, CAST(r.first_name || ' ' || r.last_name AS VARCHAR) AS rider_name
        FROM tracking.notifications n
        INNER JOIN tracking.riders r ON r.id = n.rider_id
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE n.user_id = p_user_id AND o.tenant_id = p_tenant_id
    ),
    filtered AS (
        SELECT * FROM mine WHERE NOT COALESCE(p_unread, FALSE) OR read_at IS NULL
    ),
    page AS (
        SELECT * FROM filtered
        ORDER BY created_at DESC, id
        LIMIT LEAST(GREATEST(COALESCE(p_page_size, 20), 1), 100)
        OFFSET (GREATEST(COALESCE(p_page, 1), 1) - 1) * LEAST(GREATEST(COALESCE(p_page_size, 20), 1), 100)
    )
    SELECT jsonb_build_object(
        'notifications', COALESCE((
            SELECT jsonb_agg(jsonb_build_object(
                'id', p.id, 'rider_id', p.rider_id, 'rider_name', p.rider_name, 'type', p.type,
                'payload', p.payload, 'created_at', p.created_at, 'read_at', p.read_at
            ) ORDER BY p.created_at DESC, p.id)
            FROM page p), '[]'::jsonb),
        'total_count', (SELECT count(*) FROM filtered),
        'unread_count', (SELECT count(*) FROM mine WHERE read_at IS NULL)
    );
$function$;

COMMENT ON FUNCTION tracking.sp_list_notifications(uuid,uuid,boolean,integer,integer) IS 'One page of an account''s feed, newest first, as { notifications, total_count, unread_count } (TRACK-012)';
