-- TRACK-063 down: no access log, no retention clock.

DROP FUNCTION IF EXISTS tracking.sp_prune_rider_access_log(uuid, integer);
DROP FUNCTION IF EXISTS tracking.sp_anonymise_rider_contacts(uuid, integer);
DROP FUNCTION IF EXISTS tracking.sp_list_rider_access_log(uuid, uuid, date, date, integer, integer);
DROP FUNCTION IF EXISTS tracking.fn_record_rider_access(uuid, uuid, character varying, character varying, uuid[]);
ALTER TABLE tracking.riders DROP COLUMN anonymised_at, DROP COLUMN deactivated_at;
DROP TABLE IF EXISTS tracking.rider_access_log;
