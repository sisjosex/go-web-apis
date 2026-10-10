CREATE OR REPLACE FUNCTION tracking.fn_rider_guardians_link(p_tenant_id uuid, p_rider_id uuid, p_guardians jsonb)
 RETURNS integer
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_item  RECORD;
    v_count INT := 0;
BEGIN
    FOR v_item IN
        SELECT e.value, e.ordinality - 1 AS idx
        FROM jsonb_array_elements(COALESCE(p_guardians, '[]'::jsonb)) WITH ORDINALITY AS e(value, ordinality)
    LOOP
        IF NULLIF(v_item.value->>'user_id', '') IS NOT NULL THEN
            PERFORM tracking.sp_rider_guardian_add(
                p_tenant_id, p_rider_id, (v_item.value->>'user_id')::UUID,
                v_item.value->>'name', v_item.value->>'email', v_item.value->>'phone', NULL);
        ELSIF NULLIF(TRIM(v_item.value->>'name'), '') IS NOT NULL AND NULLIF(TRIM(v_item.value->>'phone'), '') IS NOT NULL THEN
            INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, is_primary)
            VALUES (
                p_rider_id, 'guardian', TRIM(v_item.value->>'name'), TRIM(v_item.value->>'phone'),
                NOT EXISTS (
                    SELECT 1 FROM tracking.rider_contacts c
                    WHERE c.rider_id = p_rider_id AND c.relation = 'guardian' AND c.is_primary
                )
            );
        ELSE
            RAISE EXCEPTION 'rider.guardian-invalid' USING ERRCODE = 'P0001', DETAIL = 'index=' || v_item.idx;
        END IF;
        v_count := v_count + 1;
    END LOOP;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_rider_guardians_link(uuid,uuid,jsonb) IS 'Links a new rider''s guardians: granted accounts {user_id, name, email, phone} and contacts {name, phone}; raises tracking.rider.guardian-invalid with DETAIL index=<n> (TRACK-037 D2, D3)';
