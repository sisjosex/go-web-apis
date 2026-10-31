CREATE OR REPLACE FUNCTION inventory.fn_sync_axis_attributes(p_tenant_id uuid, p_product_id uuid, p_old_codes text[])
 RETURNS void
 LANGUAGE sql
AS $function$
    -- The chips lit on an axis are the product's values of that attribute, and what the filters read.
    UPDATE inventory.products p
    SET attributes = (p.attributes - COALESCE(p_old_codes, ARRAY[]::TEXT[])) || COALESCE((
        SELECT jsonb_object_agg(a.code, (
            SELECT COALESCE(jsonb_agg(vo.option_name ORDER BY vo.sort_order), '[]'::jsonb)
            FROM inventory.product_variant_options vo
            WHERE vo.variant_group_id = vg.id
        ))
        FROM inventory.product_variant_groups vg
        INNER JOIN inventory.attributes a ON a.id = vg.attribute_id
        WHERE vg.product_id = p_product_id AND vg.affects_inventory
    ), '{}'::jsonb)
    WHERE p.id = p_product_id AND p.tenant_id = p_tenant_id;
$function$;

COMMENT ON FUNCTION inventory.fn_sync_axis_attributes(uuid,uuid,text[]) IS 'Copies a product''s axes into its attribute values; the codes of axes it no longer has go (INV-024 D2).';
