-- Reverse of TRACK-016 step 2's SPs. warned_at itself belongs to the table migration and stays.
DROP FUNCTION IF EXISTS tracking.sp_raise_document_alerts(UUID);
DROP FUNCTION IF EXISTS tracking.sp_set_document_file(UUID, UUID, VARCHAR, BIGINT, VARCHAR);
