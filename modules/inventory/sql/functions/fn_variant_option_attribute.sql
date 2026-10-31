CREATE OR REPLACE FUNCTION inventory.fn_variant_option_attribute()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_attribute UUID;
BEGIN
    SELECT vg.attribute_id INTO v_attribute FROM inventory.product_variant_groups vg WHERE vg.id = NEW.variant_group_id;
    IF v_attribute IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM inventory.attribute_options ao
        WHERE ao.attribute_id = v_attribute AND inventory.fn_fold(ao.value) = inventory.fn_fold(NEW.option_name)
    ) THEN
        INSERT INTO inventory.attribute_options (attribute_id, value, sort_order)
        SELECT v_attribute, TRIM(NEW.option_name), COALESCE(MAX(ao.sort_order), -1) + 1
        FROM inventory.attribute_options ao
        WHERE ao.attribute_id = v_attribute;
    END IF;
    RETURN NULL;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_variant_option_attribute() IS 'A new option on an axis is a new value of its attribute (INV-024 D2).';

DROP TRIGGER IF EXISTS trg_variant_options_attribute ON inventory.product_variant_options;
CREATE TRIGGER trg_variant_options_attribute
    AFTER INSERT OR UPDATE OF option_name ON inventory.product_variant_options
    FOR EACH ROW EXECUTE FUNCTION inventory.fn_variant_option_attribute();
