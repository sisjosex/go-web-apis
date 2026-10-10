CREATE OR REPLACE FUNCTION tenancy.sp_free_tenant_slug(p_name character varying)
 RETURNS character varying
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_base   VARCHAR;
    v_slug   VARCHAR;
    v_suffix INT := 1;
BEGIN
    v_base := TRIM(BOTH '-' FROM REGEXP_REPLACE(
        TRANSLATE(LOWER(TRIM(COALESCE(p_name, ''))), 'áàäâãéèëêíìïîóòöôõúùüûñç', 'aaaaaeeeeiiiiooooouuuunc'),
        '[^a-z0-9]+', '-', 'g'));
    v_base := TRIM(BOTH '-' FROM LEFT(COALESCE(NULLIF(v_base, ''), 'negocio'), 90));
    v_slug := v_base;
    WHILE EXISTS (SELECT 1 FROM tenancy.tenants t WHERE t.slug = v_slug) LOOP
        v_suffix := v_suffix + 1;
        v_slug := v_base || '-' || v_suffix;
    END LOOP;
    RETURN v_slug;
END;
$function$;

COMMENT ON FUNCTION tenancy.sp_free_tenant_slug(character varying) IS 'The slug a new business of this name gets: accents out, hyphens between words, -2, -3… until free (APP-009 D3)';
