-- INFRA-007 step 6: answers the URL of the media row it deleted, so the caller can delete the
-- object behind it. The return type changes, hence DROP + CREATE.

DROP FUNCTION IF EXISTS inventory.sp_remove_product_media(UUID, UUID);
CREATE FUNCTION inventory.sp_remove_product_media(
    p_tenant_id UUID,
    p_media_id  UUID
)
RETURNS TABLE(media_url TEXT) LANGUAGE plpgsql AS $$
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
$$;
