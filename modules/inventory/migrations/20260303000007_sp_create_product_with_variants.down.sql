-- Drop SP create product with variants
DROP FUNCTION IF EXISTS inventory.sp_create_product_with_variants(VARCHAR, VARCHAR, TEXT, DECIMAL, JSONB, UUID);
