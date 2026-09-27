DROP FUNCTION IF EXISTS auth.sp_forget_push_token(TEXT);
DROP FUNCTION IF EXISTS auth.sp_push_tokens(UUID);
DROP FUNCTION IF EXISTS auth.sp_delete_push_device(UUID, VARCHAR);
DROP FUNCTION IF EXISTS auth.sp_upsert_push_device(UUID, VARCHAR, VARCHAR, TEXT);
DROP TABLE IF EXISTS auth.push_devices;
