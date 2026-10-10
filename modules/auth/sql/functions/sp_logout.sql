CREATE OR REPLACE FUNCTION auth.sp_logout(p_user_id uuid DEFAULT NULL::uuid, p_session_id uuid DEFAULT NULL::uuid, p_device_id uuid DEFAULT NULL::uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_session_count INT;
    v_session_id UUID;
BEGIN
    -- Si se proporciona p_session_id, cerrar esa sesión en particular
    IF p_session_id IS NOT NULL THEN
        -- Verificar si la sesión existe para el usuario dado
        SELECT us.session_id
        INTO v_session_id
        FROM auth.user_sessions us
        WHERE us.session_id = p_session_id
        AND us.user_id = p_user_id
        AND us.is_active = true;

        IF v_session_id IS NULL THEN
            RAISE EXCEPTION 'user.session.not-found' USING ERRCODE = 'S0001', DETAIL = 'user session does not found';
        END IF;

        -- Desactivar la sesión en lugar de eliminarla:
        UPDATE auth.user_sessions
        SET is_active = FALSE, logout_time = NOW()
        WHERE session_id = v_session_id;

    ELSIF p_device_id IS NOT NULL THEN
        -- Verificar si la sesión existe para el usuario dado
        SELECT us.session_id
        INTO v_session_id
        FROM auth.user_sessions us
        WHERE us.device_id = p_device_id
        AND us.user_id = p_user_id
        AND us.is_active = true;

        IF v_session_id IS NULL THEN
            RAISE EXCEPTION 'user.session.not-found' USING ERRCODE = 'S0001', DETAIL = 'user session does not found';
        END IF;

        -- Desactivar la sesión en lugar de eliminarla:
        UPDATE auth.user_sessions
        SET is_active = FALSE, logout_time = NOW()
        WHERE session_id = v_session_id;


    ELSE
        -- Desactivar todas las sesiones:
        UPDATE auth.user_sessions
        SET is_active = FALSE, logout_time = NOW()
        WHERE user_id = p_user_id
        AND is_active = TRUE;
    END IF;

    RETURN TRUE;

END;
$function$;
