DROP FUNCTION IF EXISTS tracking.sp_set_notification_settings(UUID, UUID, JSONB);
DROP FUNCTION IF EXISTS tracking.sp_get_notification_settings(UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_mark_notifications_read(UUID, UUID, UUID[], BOOLEAN);
DROP FUNCTION IF EXISTS tracking.sp_list_notifications(UUID, UUID, BOOLEAN, INT, INT);
DROP FUNCTION IF EXISTS tracking.fn_notice_scope(UUID, UUID);
