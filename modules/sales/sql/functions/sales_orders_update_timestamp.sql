CREATE OR REPLACE FUNCTION sales.sales_orders_update_timestamp()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$function$;

DROP TRIGGER IF EXISTS sales_orders_update_timestamp ON sales.sales_orders;
CREATE TRIGGER sales_orders_update_timestamp BEFORE UPDATE ON sales.sales_orders FOR EACH ROW EXECUTE FUNCTION sales.sales_orders_update_timestamp();
