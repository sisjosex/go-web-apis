CREATE OR REPLACE FUNCTION tracking.sp_set_notification_settings(p_tenant_id uuid, p_user_id uuid, p_settings jsonb)
 RETURNS TABLE(rider_id uuid, rider_name character varying, muted character varying[])
 LANGUAGE plpgsql
AS $function$
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
$function$;

COMMENT ON FUNCTION tracking.sp_set_notification_settings(uuid,uuid,jsonb) IS 'Replaces the muted types of each rider named in [{rider_id, muted}]; a rider outside the account''s scope raises rider.not-found (TRACK-012)';
