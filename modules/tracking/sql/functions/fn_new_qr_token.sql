CREATE OR REPLACE FUNCTION tracking.fn_new_qr_token()
 RETURNS text
 LANGUAGE plpgsql
AS $function$
DECLARE
    c_alphabet CONSTANT TEXT := 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
    v_bytes    BYTEA := gen_random_bytes(20);
    v_bits     BIGINT;
    v_token    TEXT := '';
BEGIN
    FOR g IN 0..3 LOOP
        v_bits := 0;
        FOR i IN 0..4 LOOP
            v_bits := (v_bits << 8) | get_byte(v_bytes, g * 5 + i);
        END LOOP;
        FOR j IN REVERSE 7..0 LOOP
            v_token := v_token || substr(c_alphabet, ((v_bits >> (j * 5)) & 31)::INT + 1, 1);
        END LOOP;
    END LOOP;
    RETURN v_token;
END;
$function$;

COMMENT ON FUNCTION tracking.fn_new_qr_token() IS 'A new rider card token: 20 random bytes as 32 base32 characters (TRACK-027 D1)';
