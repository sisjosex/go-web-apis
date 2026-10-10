CREATE OR REPLACE FUNCTION tracking.fn_outbox_notify()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    PERFORM pg_notify('tracking_outbox', '');
    RETURN NULL;
END;
$function$;

DROP TRIGGER IF EXISTS trg_outbox_notify ON tracking.outbox;
CREATE TRIGGER trg_outbox_notify AFTER INSERT ON tracking.outbox FOR EACH STATEMENT EXECUTE FUNCTION tracking.fn_outbox_notify();
