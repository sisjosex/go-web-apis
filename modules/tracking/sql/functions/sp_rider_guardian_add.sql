CREATE OR REPLACE FUNCTION tracking.sp_rider_guardian_add(p_tenant_id uuid, p_rider_id uuid, p_user_id uuid, p_name character varying, p_email character varying, p_phone character varying, p_scope_user_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    PERFORM tracking.fn_require_rider(p_tenant_id, p_rider_id, p_scope_user_id, true);

    IF EXISTS (
        SELECT 1 FROM tracking.rider_contacts rc
        WHERE rc.rider_id = p_rider_id AND rc.relation = 'guardian' AND rc.user_id = p_user_id
    ) THEN
        RETURN;
    END IF;

    -- The guardian the rider form already names by this email — or phone (TRACK-062) — becomes the
    -- account's row.
    UPDATE tracking.rider_contacts rc SET
        user_id = p_user_id,
        updated_at = CURRENT_TIMESTAMP
    WHERE rc.id = (
        SELECT c.id FROM tracking.rider_contacts c
        WHERE c.rider_id = p_rider_id AND c.relation = 'guardian' AND c.user_id IS NULL
          AND (LOWER(c.email) = LOWER(TRIM(p_email)) OR c.phone = NULLIF(TRIM(p_phone), ''))
        ORDER BY (LOWER(c.email) = LOWER(TRIM(p_email))) DESC NULLS LAST, c.is_primary DESC
        LIMIT 1
    );
    IF FOUND THEN
        RETURN;
    END IF;

    INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, email, user_id, is_primary)
    VALUES (
        p_rider_id, 'guardian',
        NULLIF(TRIM(p_name), ''), NULLIF(TRIM(p_phone), ''), NULLIF(LOWER(TRIM(p_email)), ''),
        p_user_id,
        NOT EXISTS (
            SELECT 1 FROM tracking.rider_contacts c
            WHERE c.rider_id = p_rider_id AND c.relation = 'guardian' AND c.is_primary
        )
    );
END;
$function$;

COMMENT ON FUNCTION tracking.sp_rider_guardian_add(uuid,uuid,uuid,character varying,character varying,character varying,uuid) IS 'Links a guardian account to a rider (idempotent), onto the contact row naming its email or phone when there is one (TRACK-062); the access level was granted by tenancy.sp_grant_app_access first';
