DROP FUNCTION IF EXISTS tenancy.sp_list_usage_daily(DATE, DATE, UUID);
DROP FUNCTION IF EXISTS tenancy.sp_usage_daily_upsert(UUID, DATE, INT, INT, BIGINT, INT);
DROP TABLE IF EXISTS tenancy.usage_daily;
