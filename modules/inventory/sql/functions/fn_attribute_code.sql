CREATE OR REPLACE FUNCTION inventory.fn_attribute_code(p_name text)
 RETURNS text
 LANGUAGE plpgsql
 IMMUTABLE
AS $function$
BEGIN
    RETURN trim(BOTH '-' FROM regexp_replace(inventory.fn_fold(p_name), '[^a-z0-9]+', '-', 'g'));
END;
$function$;

COMMENT ON FUNCTION inventory.fn_attribute_code(text) IS 'The code an attribute name becomes: Talla de calzado is talla-de-calzado, and TALLA is talla (INV-024 D2).';
