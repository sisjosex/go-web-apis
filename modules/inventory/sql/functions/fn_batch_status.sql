CREATE OR REPLACE FUNCTION inventory.fn_batch_status(p_expiry_date date, p_status character varying, p_today date)
 RETURNS character varying
 LANGUAGE plpgsql
 IMMUTABLE
AS $function$
BEGIN
    -- The two values an explicit action writes; everything else comes from the date.
    IF p_status IN ('void', 'depleted')
    THEN
        RETURN p_status;
    END IF;

    IF p_expiry_date < p_today
    THEN
        RETURN 'expired';
    END IF;

    IF p_expiry_date <= p_today + 7
    THEN
        RETURN 'expiring_soon';
    END IF;

    RETURN 'active';
END;
$function$;

COMMENT ON FUNCTION inventory.fn_batch_status(date,character varying,date) IS 'A lot''s status on p_today (the tenant''s own date, INV-018 D1): void and depleted are stored facts and win; otherwise expired past its expiry date, expiring_soon within 7 days, else active. Derived on every read, so no job keeps it fresh (D2).';
