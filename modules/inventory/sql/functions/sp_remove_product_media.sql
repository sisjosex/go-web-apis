CREATE OR REPLACE FUNCTION inventory.sp_remove_product_media(p_tenant_id uuid, p_media_id uuid)
 RETURNS TABLE(media_url text)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_url TEXT;
BEGIN
    DELETE FROM inventory.product_media pm
    USING inventory.products p
    WHERE pm.id        = p_media_id
      AND pm.product_id = p.id
      AND p.tenant_id  = p_tenant_id
    RETURNING pm.url INTO v_url;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'I0002: Media not found';
    END IF;

    RETURN QUERY SELECT v_url;
END;
$function$;
