CREATE OR REPLACE FUNCTION inventory.fn_products_in_scope(p_tenant_id uuid, p_category_id uuid, p_search text, p_filter jsonb)
 RETURNS TABLE(id uuid)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_folded   TEXT := NULLIF(inventory.fn_fold(TRIM(COALESCE(p_search, ''))), '');
    v_patterns TEXT[];
    v_longest  TEXT;
    v_fuzzy    BOOLEAN := FALSE;
    v_filter   JSONB := COALESCE(p_filter, '{}'::jsonb);
    v_single   JSONB;
    v_several  JSONB;
BEGIN
    -- Several words, each must appear; the longest one reaches the trigram index, the others read what it found (D4).
    IF v_folded IS NOT NULL THEN
        SELECT array_agg('%' || replace(replace(replace(w, '\', '\\'), '%', '\%'), '_', '\_') || '%' ORDER BY length(w) DESC)
        INTO v_patterns
        FROM regexp_split_to_table(v_folded, '\s+') w;
        v_longest := v_patterns[1];
        v_fuzzy := NOT EXISTS (
            SELECT 1 FROM inventory.products p
            WHERE p.tenant_id = p_tenant_id AND p.search_text LIKE v_longest AND p.search_text LIKE ALL (v_patterns)
        );
    END IF;

    -- One value per attribute goes in a single containment; several values of one attribute read what that found (D1).
    SELECT COALESCE(jsonb_object_agg(f.key, f.value) FILTER (WHERE jsonb_array_length(f.value) = 1), '{}'::jsonb),
           COALESCE(jsonb_object_agg(f.key, f.value) FILTER (WHERE jsonb_array_length(f.value) > 1), '{}'::jsonb)
    INTO v_single, v_several
    FROM jsonb_each(v_filter) f;

    RETURN QUERY
    SELECT p.id
    FROM inventory.products p
    WHERE p.tenant_id = p_tenant_id
      AND (
          p_category_id IS NULL
          OR EXISTS (
              SELECT 1 FROM inventory.product_category_mapping m
              WHERE m.product_id = p.id AND m.category_id = p_category_id
          )
      )
      AND (v_single = '{}'::jsonb OR p.attributes @> v_single)
      AND (v_several = '{}'::jsonb OR NOT EXISTS (
          SELECT 1 FROM jsonb_each(v_several) f
          WHERE NOT COALESCE(p.attributes -> f.key ?| ARRAY(SELECT jsonb_array_elements_text(f.value)), FALSE)
      ))
      AND (
          v_folded IS NULL
          OR (NOT v_fuzzy AND p.search_text LIKE v_longest AND p.search_text LIKE ALL (v_patterns))
          -- Only when the exact words found nothing: tuvo pvc finds Tubo PVC.
          OR (v_fuzzy AND word_similarity(v_folded, p.search_text) >= 0.4)
      );
END;
$function$;

COMMENT ON FUNCTION inventory.fn_products_in_scope(uuid,uuid,text,jsonb) IS 'The products a list reads: by category, attribute filter and search, every word present, with one retry by likeness when nothing matches (INV-024 D1, D4).';
