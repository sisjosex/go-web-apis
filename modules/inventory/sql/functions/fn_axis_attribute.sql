CREATE OR REPLACE FUNCTION inventory.fn_axis_attribute(p_tenant_id uuid, p_attribute_id uuid, p_name text, p_options jsonb)
 RETURNS uuid
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_id     UUID := p_attribute_id;
    v_code   TEXT;
    v_option TEXT;
    v_sort   INT;
BEGIN
    IF v_id IS NOT NULL THEN
        SELECT a.code INTO v_code FROM inventory.attributes a WHERE a.id = v_id AND a.tenant_id = p_tenant_id;
        IF v_code IS NULL THEN
            RAISE EXCEPTION 'inventory.attribute.not-found' USING ERRCODE = 'P0001';
        END IF;
        IF EXISTS (
            SELECT 1 FROM jsonb_array_elements(COALESCE(p_options, '[]'::jsonb)) o
            WHERE NOT EXISTS (
                SELECT 1 FROM inventory.attribute_options ao
                WHERE ao.attribute_id = v_id AND inventory.fn_fold(ao.value) = inventory.fn_fold(o ->> 'name')
            )
        ) THEN
            RAISE EXCEPTION 'inventory.attribute.invalid' USING ERRCODE = 'P0001', DETAIL = v_code;
        END IF;
        RETURN v_id;
    END IF;

    -- A name, not an id: the attribute of that name, created on the fly, and its new options with it (D2).
    v_code := inventory.fn_attribute_code(p_name);
    INSERT INTO inventory.attributes (tenant_id, code, name)
    VALUES (p_tenant_id, v_code, TRIM(p_name))
    ON CONFLICT (tenant_id, code) DO NOTHING;
    SELECT a.id INTO v_id FROM inventory.attributes a WHERE a.tenant_id = p_tenant_id AND a.code = v_code;

    SELECT COALESCE(MAX(ao.sort_order), -1) + 1 INTO v_sort FROM inventory.attribute_options ao WHERE ao.attribute_id = v_id;
    FOR v_option IN SELECT TRIM(o ->> 'name') FROM jsonb_array_elements(COALESCE(p_options, '[]'::jsonb)) o LOOP
        IF NOT EXISTS (
            SELECT 1 FROM inventory.attribute_options ao
            WHERE ao.attribute_id = v_id AND inventory.fn_fold(ao.value) = inventory.fn_fold(v_option)
        ) THEN
            INSERT INTO inventory.attribute_options (attribute_id, value, sort_order) VALUES (v_id, v_option, v_sort);
            v_sort := v_sort + 1;
        END IF;
    END LOOP;
    RETURN v_id;
END;
$function$;

COMMENT ON FUNCTION inventory.fn_axis_attribute(uuid,uuid,text,jsonb) IS 'The attribute behind a variant axis: the one named by id, whose options the axis must use, or the one of that name, created on the fly (INV-024 D2).';
