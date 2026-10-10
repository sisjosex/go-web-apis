CREATE OR REPLACE FUNCTION tracking.fn_upsert_primary_rider_contact(p_rider_id uuid, p_relation character varying, p_name character varying, p_phone character varying, p_email character varying, p_user_id uuid)
 RETURNS void
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF p_name IS NULL AND p_phone IS NULL AND p_email IS NULL AND p_user_id IS NULL THEN
        RETURN;
    END IF;

    UPDATE tracking.rider_contacts c SET
        name = CASE WHEN p_name IS NULL THEN c.name ELSE NULLIF(TRIM(p_name), '') END,
        phone = CASE WHEN p_phone IS NULL THEN c.phone ELSE NULLIF(TRIM(p_phone), '') END,
        email = CASE WHEN p_email IS NULL THEN c.email ELSE NULLIF(TRIM(p_email), '') END,
        user_id = COALESCE(p_user_id, c.user_id),
        updated_at = CURRENT_TIMESTAMP
    WHERE c.rider_id = p_rider_id AND c.relation = p_relation AND c.is_primary;

    IF NOT FOUND AND (
        NULLIF(TRIM(p_name), '') IS NOT NULL OR NULLIF(TRIM(p_phone), '') IS NOT NULL
        OR NULLIF(TRIM(p_email), '') IS NOT NULL OR p_user_id IS NOT NULL
    ) THEN
        INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, email, user_id, is_primary)
        VALUES (
            p_rider_id, p_relation,
            NULLIF(TRIM(p_name), ''), NULLIF(TRIM(p_phone), ''), NULLIF(TRIM(p_email), ''),
            p_user_id, true
        );
    END IF;
END;
$function$;
