-- BILLING-001 down.
DROP FUNCTION IF EXISTS public.sp_get_tenant_usage(UUID);
DROP FUNCTION IF EXISTS public.fn_within_limit(UUID, BIGINT, INT, TEXT);
