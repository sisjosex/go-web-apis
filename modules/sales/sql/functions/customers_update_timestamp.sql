CREATE OR REPLACE FUNCTION sales.customers_update_timestamp()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS customers_update_timestamp ON sales.customers;
CREATE TRIGGER customers_update_timestamp BEFORE UPDATE ON sales.customers FOR EACH ROW EXECUTE FUNCTION sales.customers_update_timestamp();
