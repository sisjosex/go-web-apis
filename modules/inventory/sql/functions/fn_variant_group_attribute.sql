CREATE OR REPLACE FUNCTION inventory.fn_variant_group_attribute()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_tenant UUID;
BEGIN
    IF NEW.affects_inventory AND NEW.attribute_id IS NULL THEN
        SELECT p.tenant_id INTO v_tenant FROM inventory.products p WHERE p.id = NEW.product_id;
        NEW.attribute_id := inventory.fn_axis_attribute(v_tenant, NULL, NEW.group_type, NULL);
    END IF;
    RETURN NEW;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_variant_group_attribute() IS 'An axis that names no attribute takes the one of its name, created on the fly: no writer can leave an axis unbound (INV-024 D2).';

-- D2 for every writer of an axis, the import's included: the row finds its attribute by name.
DROP TRIGGER IF EXISTS trg_variant_groups_attribute ON inventory.product_variant_groups;
CREATE TRIGGER trg_variant_groups_attribute
    BEFORE INSERT OR UPDATE OF affects_inventory, group_type ON inventory.product_variant_groups
    FOR EACH ROW EXECUTE FUNCTION inventory.fn_variant_group_attribute();
