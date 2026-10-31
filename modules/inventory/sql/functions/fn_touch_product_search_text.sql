CREATE OR REPLACE FUNCTION inventory.fn_touch_product_search_text()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_sku_id     UUID;
    v_product_id UUID;
BEGIN
    IF TG_TABLE_NAME = 'product_skus' THEN
        v_product_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.product_id ELSE NEW.product_id END;
    ELSE
        v_sku_id := CASE WHEN TG_OP = 'DELETE' THEN OLD.sku_id ELSE NEW.sku_id END;
        SELECT s.product_id INTO v_product_id FROM inventory.product_skus s WHERE s.id = v_sku_id;
    END IF;

    -- search_text alone does not fire the product's own trigger.
    UPDATE inventory.products p
    SET search_text = inventory.fn_product_search_text(p.tenant_id, p.id, p.name, p.sku, p.attributes)
    WHERE p.id = v_product_id;
    RETURN NULL;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_touch_product_search_text() IS 'A combination or barcode that changes rewrites its product''s search_text, whichever SP wrote it (INV-024 D4).';

DROP TRIGGER IF EXISTS trg_skus_search_text ON inventory.product_skus;
CREATE TRIGGER trg_skus_search_text
    AFTER INSERT OR DELETE OR UPDATE OF sku ON inventory.product_skus
    FOR EACH ROW EXECUTE FUNCTION inventory.fn_touch_product_search_text();

DROP TRIGGER IF EXISTS trg_sku_codes_search_text ON inventory.sku_codes;
CREATE TRIGGER trg_sku_codes_search_text
    AFTER INSERT OR DELETE ON inventory.sku_codes
    FOR EACH ROW EXECUTE FUNCTION inventory.fn_touch_product_search_text();
