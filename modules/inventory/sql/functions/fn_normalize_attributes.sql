CREATE OR REPLACE FUNCTION inventory.fn_normalize_attributes(p_tenant_id uuid, p_values jsonb)
 RETURNS jsonb
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_entry  RECORD;
    v_attr   inventory.attributes%ROWTYPE;
    v_value  TEXT;
    v_values TEXT[];
    v_option TEXT;
    v_out    JSONB := '{}'::jsonb;
BEGIN
    IF p_values IS NULL OR jsonb_typeof(p_values) <> 'object' THEN
        RETURN '{}'::jsonb;
    END IF;

    FOR v_entry IN SELECT e.key, e.value FROM jsonb_each(p_values) e LOOP
        SELECT a.* INTO v_attr FROM inventory.attributes a WHERE a.tenant_id = p_tenant_id AND a.code = v_entry.key;
        IF v_attr.id IS NULL THEN
            RAISE EXCEPTION 'inventory.attribute.invalid' USING ERRCODE = 'P0001', DETAIL = v_entry.key;
        END IF;

        v_values := ARRAY[]::TEXT[];
        FOR v_value IN
            SELECT TRIM(x) FROM jsonb_array_elements_text(
                CASE jsonb_typeof(v_entry.value)
                    WHEN 'array' THEN v_entry.value
                    WHEN 'null' THEN '[]'::jsonb
                    ELSE jsonb_build_array(v_entry.value)
                END
            ) x
            WHERE TRIM(x) <> ''
        LOOP
            IF v_attr.type = 'number' AND v_value !~ '^-?[0-9]+([.,][0-9]+)?$' THEN
                RAISE EXCEPTION 'inventory.attribute.invalid' USING ERRCODE = 'P0001', DETAIL = v_entry.key;
            END IF;
            IF v_attr.type IN ('list', 'color', 'measure') THEN
                SELECT o.value INTO v_option
                FROM inventory.attribute_options o
                WHERE o.attribute_id = v_attr.id AND inventory.fn_fold(o.value) = inventory.fn_fold(v_value);
                IF v_option IS NULL AND (v_attr.type <> 'measure' OR EXISTS (
                    SELECT 1 FROM inventory.attribute_options o WHERE o.attribute_id = v_attr.id
                )) THEN
                    RAISE EXCEPTION 'inventory.attribute.invalid' USING ERRCODE = 'P0001', DETAIL = v_entry.key;
                END IF;
                v_value := COALESCE(v_option, v_value);
            END IF;
            v_values := v_values || v_value;
        END LOOP;

        IF cardinality(v_values) > 0 THEN
            v_out := v_out || jsonb_build_object(v_entry.key, to_jsonb(v_values));
        END IF;
    END LOOP;
    RETURN v_out;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_normalize_attributes(uuid,jsonb) IS 'Checks attribute values against their definitions and answers each as a list of canonical strings. Raises inventory.attribute.invalid with the code in the detail (INV-024).';
