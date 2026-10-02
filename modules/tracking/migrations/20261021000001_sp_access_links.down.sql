DROP FUNCTION IF EXISTS tracking.sp_driver_account_set(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_rider_guardian_remove(UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_rider_guardian_add(UUID, UUID, UUID, VARCHAR, VARCHAR, VARCHAR, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_guardians(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_require_rider(UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.fn_user_linked(UUID, UUID);
