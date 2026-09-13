DROP FUNCTION IF EXISTS inventory.sp_import_free_category_slug(UUID, VARCHAR, INT);
DROP FUNCTION IF EXISTS inventory.sp_import_sku_exists(UUID, VARCHAR);
DROP FUNCTION IF EXISTS inventory.sp_import_find_variant_option(UUID, UUID, VARCHAR, VARCHAR);
DROP FUNCTION IF EXISTS inventory.sp_import_find_category(UUID, VARCHAR, UUID);
DROP FUNCTION IF EXISTS inventory.sp_import_resolve_sku_by_axes(UUID, VARCHAR, TEXT[], TEXT[]);
