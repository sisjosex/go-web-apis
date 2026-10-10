CREATE OR REPLACE FUNCTION tracking.fn_assignment_on(p_days_of_week smallint, p_valid_from date, p_valid_until date, p_date date)
 RETURNS boolean
 LANGUAGE sql
 IMMUTABLE
AS $function$
    SELECT p_date >= p_valid_from
       AND (p_valid_until IS NULL OR p_date <= p_valid_until)
       AND ((p_days_of_week >> (CAST(EXTRACT(ISODOW FROM p_date) AS INT) - 1)) & 1) = 1;
$function$;

COMMENT ON FUNCTION tracking.fn_assignment_on(smallint,date,date,date) IS 'Whether an assignment with these days and validity is in force on p_date (TRACK-009)';
