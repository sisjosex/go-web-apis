CREATE OR REPLACE FUNCTION inventory.fn_products_search_text()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
    NEW.search_text := inventory.fn_product_search_text(NEW.tenant_id, NEW.id, NEW.name, NEW.sku, NEW.attributes);
    RETURN NEW;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_products_search_text() IS 'Keeps search_text on the product row whenever its name, code or attributes change (INV-024 D4).';

DROP TRIGGER IF EXISTS trg_products_search_text ON inventory.products;
CREATE TRIGGER trg_products_search_text
    BEFORE INSERT OR UPDATE OF name, sku, attributes ON inventory.products
    FOR EACH ROW EXECUTE FUNCTION inventory.fn_products_search_text();
