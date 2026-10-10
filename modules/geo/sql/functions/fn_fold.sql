CREATE OR REPLACE FUNCTION geo.fn_fold(p_text text)
 RETURNS text
 LANGUAGE sql
 IMMUTABLE PARALLEL SAFE
AS $function$
    SELECT lower(public.unaccent('public.unaccent'::regdictionary, p_text))
$function$;

COMMENT ON FUNCTION geo.fn_fold(text) IS 'Lowercase, accents removed: what geo.places.search holds and every search is folded to.';
