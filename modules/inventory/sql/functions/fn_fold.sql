CREATE OR REPLACE FUNCTION inventory.fn_fold(p_text text)
 RETURNS text
 LANGUAGE sql
 IMMUTABLE
AS $function$
    SELECT lower(translate(p_text, 'ÁÉÍÓÚÜÑáéíóúüñ', 'AEIOUUNaeiouun'));
$function$;

COMMENT ON FUNCTION inventory.fn_fold(text) IS 'Text without case or accents, as names and searches compare (INV-024 D2, D4).';
