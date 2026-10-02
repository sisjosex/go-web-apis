-- Restores sp_remove_product_media as it stood before INFRA-007: a message, no URL.

DROP FUNCTION IF EXISTS inventory.sp_remove_product_media(UUID, UUID);
CREATE FUNCTION inventory.sp_remove_product_media(
    p_tenant_id UUID,
    p_media_id  UUID
)
RETURNS TABLE(message VARCHAR) LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM inventory.product_media pm
    USING inventory.products p
    WHERE pm.id        = p_media_id
      AND pm.product_id = p.id
      AND p.tenant_id  = p_tenant_id;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'I0002: Media not found';
    END IF;

    RETURN QUERY SELECT 'Media removed successfully'::VARCHAR;
END;
$$;
