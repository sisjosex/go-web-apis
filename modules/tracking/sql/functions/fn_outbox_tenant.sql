CREATE OR REPLACE FUNCTION tracking.fn_outbox_tenant()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    IF NEW.tenant_id IS NULL THEN
        NEW.tenant_id := COALESCE(
            CAST(NEW.payload->>'tenant_id' AS UUID),
            (SELECT tc.tenant_id
               FROM tracking.routes r
               INNER JOIN tracking.transport_companies tc ON tc.id = r.company_id
              WHERE r.id = CAST(NEW.payload->>'route_id' AS UUID)));
    END IF;
    IF NEW.tenant_id IS NULL THEN
        RAISE EXCEPTION 'outbox.tenant' USING ERRCODE = 'P0001',
            DETAIL = format('topic %s names no tenant_id or known route_id', NEW.topic);
    END IF;
    RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS trg_outbox_tenant ON tracking.outbox;
CREATE TRIGGER trg_outbox_tenant BEFORE INSERT ON tracking.outbox FOR EACH ROW EXECUTE FUNCTION tracking.fn_outbox_tenant();
