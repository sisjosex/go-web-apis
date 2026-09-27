-- TRACK-012 step 1: the portal's feed and settings reads and writes.
--
-- A notice goes to every account that is a rider's guardian or the rider itself (rider_contacts
-- relation guardian|self), so that is also who may read and mute it: fn_notice_scope is
-- fn_guardian_scope widened to `self`. Feed rows already carry their recipient, so the feed filters
-- on user_id; the join out to the rider's organization is the tenant guard. Every SP is one
-- statement over indexed rows — no call costs more than the page it answers.

CREATE FUNCTION tracking.fn_notice_scope(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS UUID[]
LANGUAGE sql
STABLE
AS $$
    SELECT COALESCE(ARRAY_AGG(DISTINCT rc.rider_id), ARRAY[]::UUID[])
    FROM tracking.rider_contacts rc
    INNER JOIN tracking.riders r ON r.id = rc.rider_id
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE rc.user_id = p_user_id
      AND rc.relation IN ('guardian', 'self')
      AND o.tenant_id = p_tenant_id;
$$;

COMMENT ON FUNCTION tracking.fn_notice_scope(UUID, UUID) IS
'The rider ids an account receives notices for in this tenant: those it is a guardian or self contact of (TRACK-012)';

-- One page of the caller's feed, newest first, as one object { notifications, total_count,
-- unread_count }: total_count is the filtered total, unread_count the whole badge whatever p_unread
-- says — a page past the end still carries both.
CREATE FUNCTION tracking.sp_list_notifications(
    p_tenant_id UUID,
    p_user_id UUID,
    p_unread BOOLEAN DEFAULT FALSE,
    p_page INT DEFAULT 1,
    p_page_size INT DEFAULT 20
)
RETURNS JSONB AS $$
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
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION tracking.sp_list_notifications(UUID, UUID, BOOLEAN, INT, INT) IS
'One page of an account''s feed, newest first, as { notifications, total_count, unread_count } (TRACK-012)';

-- Marks the caller's rows read: p_ids, or every unread row when p_all. Someone else's id is ignored,
-- not refused — the feed never says whose a row is. Answers how many rows changed.
CREATE FUNCTION tracking.sp_mark_notifications_read(
    p_tenant_id UUID,
    p_user_id UUID,
    p_ids UUID[],
    p_all BOOLEAN DEFAULT FALSE
)
RETURNS INT AS $$
DECLARE
    v_count INT;
BEGIN
    UPDATE tracking.notifications n
    SET read_at = now()
    FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE r.id = n.rider_id
      AND o.tenant_id = p_tenant_id
      AND n.user_id = p_user_id
      AND n.read_at IS NULL
      AND (COALESCE(p_all, FALSE) OR n.id = ANY(COALESCE(p_ids, '{}')));
    GET DIAGNOSTICS v_count = ROW_COUNT;
    RETURN v_count;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_mark_notifications_read(UUID, UUID, UUID[], BOOLEAN) IS
'Stamps read_at on the account''s unread rows in p_ids, or on all of them when p_all; others'' ids are ignored (TRACK-012)';

-- One row per rider in the caller's notice scope, with the types it muted (empty = all on).
CREATE FUNCTION tracking.sp_get_notification_settings(
    p_tenant_id UUID,
    p_user_id UUID
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    muted VARCHAR[]
) AS $$
BEGIN
    RETURN QUERY
    SELECT r.id,
           CAST(r.first_name || ' ' || r.last_name AS VARCHAR),
           COALESCE(
               (SELECT ARRAY_AGG(CAST(s.type AS VARCHAR) ORDER BY s.type)
                FROM tracking.notification_settings s
                WHERE s.user_id = p_user_id AND s.rider_id = r.id),
               ARRAY[]::VARCHAR[])
    FROM tracking.riders r
    WHERE r.id = ANY(tracking.fn_notice_scope(p_tenant_id, p_user_id))
    ORDER BY r.first_name, r.last_name, r.id;
END;
$$ LANGUAGE plpgsql STABLE;

COMMENT ON FUNCTION tracking.sp_get_notification_settings(UUID, UUID) IS
'Each rider an account receives notices for, with the types it muted (TRACK-012)';

-- Replaces the muted list of each rider p_settings names ([{rider_id, muted: [type]}]); riders it
-- does not name keep theirs. A rider outside the caller's scope is rider.not-found and nothing is
-- written. Answers the whole settings list.
CREATE FUNCTION tracking.sp_set_notification_settings(
    p_tenant_id UUID,
    p_user_id UUID,
    p_settings JSONB
)
RETURNS TABLE(
    rider_id UUID,
    rider_name VARCHAR,
    muted VARCHAR[]
) AS $$
DECLARE
    v_scope UUID[] := tracking.fn_notice_scope(p_tenant_id, p_user_id);
    v_riders UUID[];
BEGIN
    SELECT COALESCE(ARRAY_AGG(CAST(e->>'rider_id' AS UUID)), '{}')
    INTO v_riders
    FROM jsonb_array_elements(p_settings) e;

    IF EXISTS (SELECT 1 FROM unnest(v_riders) x WHERE NOT (x = ANY(v_scope))) THEN
        RAISE EXCEPTION 'rider.not-found' USING ERRCODE = 'P0001';
    END IF;

    DELETE FROM tracking.notification_settings s
    WHERE s.user_id = p_user_id AND s.rider_id = ANY(v_riders);

    INSERT INTO tracking.notification_settings (user_id, rider_id, type)
    SELECT DISTINCT p_user_id, CAST(e->>'rider_id' AS UUID), m.type
    FROM jsonb_array_elements(p_settings) e
    CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(e->'muted', '[]'::jsonb)) AS m(type);

    RETURN QUERY SELECT * FROM tracking.sp_get_notification_settings(p_tenant_id, p_user_id);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION tracking.sp_set_notification_settings(UUID, UUID, JSONB) IS
'Replaces the muted types of each rider named in [{rider_id, muted}]; a rider outside the account''s scope raises rider.not-found (TRACK-012)';
