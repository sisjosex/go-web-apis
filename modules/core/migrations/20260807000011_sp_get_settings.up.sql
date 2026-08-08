CREATE OR REPLACE FUNCTION core.sp_get_settings()
RETURNS TABLE (
    module  VARCHAR,
    section VARCHAR,
    key     VARCHAR,
    value   TEXT
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT
        CAST(s.module AS VARCHAR),
        CAST(s.section AS VARCHAR),
        CAST(s.key AS VARCHAR),
        CAST(s.value AS TEXT)
    FROM core.settings s
    ORDER BY s.module, s.section, s.key;
END;
$$;

COMMENT ON FUNCTION core.sp_get_settings() IS
'Returns every configuration value, ordered by module/section/key, for the boot-time overlay.';
