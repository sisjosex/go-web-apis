CREATE OR REPLACE FUNCTION inventory.sp_upsert_attribute(p_tenant_id uuid, p_attribute_id uuid, p_name character varying, p_type character varying, p_unit character varying, p_filterable boolean, p_options jsonb, p_axis boolean DEFAULT NULL::boolean)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id     UUID := p_attribute_id;
    v_code   TEXT := inventory.fn_attribute_code(p_name);
    v_option JSONB;
    v_opt_id UUID;
    v_kept   UUID[] := ARRAY[]::UUID[];
    v_sort   INT := 0;
BEGIN
    IF v_code = '' THEN
        RAISE EXCEPTION 'inventory.attribute.invalid' USING ERRCODE = 'P0001', DETAIL = 'name';
    END IF;

    BEGIN
        IF v_id IS NULL THEN
            INSERT INTO inventory.attributes (tenant_id, code, name, type, unit, filterable, axis)
            VALUES (p_tenant_id, v_code, TRIM(p_name), p_type, p_unit, COALESCE(p_filterable, TRUE), COALESCE(p_axis, TRUE))
            RETURNING id INTO v_id;
        ELSE
            -- A new name moves the values the products hold under the new code.
            UPDATE inventory.products p
            SET attributes = (p.attributes - a.code) || jsonb_build_object(v_code, p.attributes -> a.code)
            FROM inventory.attributes a
            WHERE a.id = v_id AND a.tenant_id = p_tenant_id AND a.code <> v_code
              AND p.tenant_id = p_tenant_id AND p.attributes ? a.code;

            UPDATE inventory.attributes a
            SET code = v_code, name = TRIM(p_name), type = p_type, unit = p_unit, filterable = COALESCE(p_filterable, a.filterable), axis = COALESCE(p_axis, a.axis)
            WHERE a.id = v_id AND a.tenant_id = p_tenant_id;
            IF NOT FOUND THEN
                RAISE EXCEPTION 'inventory.attribute.not-found' USING ERRCODE = 'P0001';
            END IF;
        END IF;
    EXCEPTION WHEN unique_violation THEN
        RAISE EXCEPTION 'inventory.attribute.duplicate' USING ERRCODE = 'P0001';
    END;

    FOR v_option IN SELECT * FROM jsonb_array_elements(COALESCE(p_options, '[]'::jsonb)) LOOP
        v_opt_id := NULLIF(v_option ->> 'id', '')::UUID;
        IF v_opt_id IS NULL THEN
            INSERT INTO inventory.attribute_options (attribute_id, value, swatch, sort_order)
            VALUES (v_id, TRIM(v_option ->> 'value'), v_option ->> 'swatch', COALESCE((v_option ->> 'sort')::INT, v_sort))
            ON CONFLICT (attribute_id, value) DO UPDATE SET swatch = EXCLUDED.swatch, sort_order = EXCLUDED.sort_order
            RETURNING id INTO v_opt_id;
        ELSE
            UPDATE inventory.attribute_options o
            SET value = TRIM(v_option ->> 'value'), swatch = v_option ->> 'swatch', sort_order = COALESCE((v_option ->> 'sort')::INT, v_sort)
            WHERE o.id = v_opt_id AND o.attribute_id = v_id;
        END IF;
        v_kept := v_kept || v_opt_id;
        v_sort := v_sort + 1;
    END LOOP;

    -- An option a product still holds stays: taking it away would leave a value nothing defines.
    IF EXISTS (
        SELECT 1
        FROM inventory.attribute_options o
        INNER JOIN inventory.products p ON p.tenant_id = p_tenant_id AND p.attributes @> jsonb_build_object(v_code, jsonb_build_array(o.value))
        WHERE o.attribute_id = v_id AND o.id <> ALL (v_kept)
    ) THEN
        RAISE EXCEPTION 'inventory.attribute.in-use' USING ERRCODE = 'P0001';
    END IF;
    DELETE FROM inventory.attribute_options o WHERE o.attribute_id = v_id AND o.id <> ALL (v_kept);

    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION inventory.sp_upsert_attribute(uuid,uuid,character varying,character varying,character varying,boolean,jsonb,boolean) IS 'Creates or changes an attribute and its whole list of options, diffed by id. Raises inventory.attribute.duplicate, inventory.attribute.not-found, and inventory.attribute.in-use for an option a product holds.';
