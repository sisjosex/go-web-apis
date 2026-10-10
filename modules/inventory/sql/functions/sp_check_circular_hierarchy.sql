CREATE OR REPLACE FUNCTION inventory.sp_check_circular_hierarchy(p_tenant_id uuid, p_category_id uuid, p_parent_id uuid)
 RETURNS boolean
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_current_id UUID := p_parent_id;
    v_iterations INT := 0;
BEGIN
    -- Prevent infinite loops
    WHILE v_current_id IS NOT NULL AND v_iterations < 100 LOOP
        IF v_current_id = p_category_id THEN
            RETURN FALSE;  -- Circular reference detected
        END IF;
        SELECT c.parent_id INTO v_current_id
        FROM inventory.product_categories c
        WHERE c.id = v_current_id AND c.tenant_id = p_tenant_id AND c.deleted_at IS NULL;
        v_iterations := v_iterations + 1;
    END LOOP;
    RETURN TRUE;  -- No circular reference
END;
$function$;
