CREATE OR REPLACE FUNCTION inventory.fn_attribute_filter(p_tenant_id uuid, p_attrs jsonb)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
BEGIN
    -- attr[marca]=truper reads the option's own spelling, Truper, which is what the containment compares.
    RETURN (SELECT COALESCE(jsonb_object_agg(f.key, (
        SELECT jsonb_agg(COALESCE((
            SELECT o.value FROM inventory.attribute_options o
            INNER JOIN inventory.attributes a ON a.id = o.attribute_id
            WHERE a.tenant_id = p_tenant_id AND a.code = f.key AND inventory.fn_fold(o.value) = inventory.fn_fold(v)
            LIMIT 1
        ), v))
        FROM jsonb_array_elements_text(f.value) v
    )), '{}'::jsonb)
    FROM jsonb_each(COALESCE(p_attrs, '{}'::jsonb)) f);
END;
$function$;

COMMENT ON FUNCTION inventory.fn_attribute_filter(uuid,jsonb) IS 'A list''s attribute filter, {code: [values]}, in the spelling the products store (INV-024).';
