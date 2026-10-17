-- TRACK-072: the pre-tenancy overload wrote riders.company_id, gone since 20260919000003_riders_organization, and
-- it was tenant-unscoped. Dropped here because removing it from sql/functions drops nothing.
DROP FUNCTION IF EXISTS tracking.sp_create_rider(uuid, character varying, character varying, character varying,
    character varying, character varying, character varying, character varying, character varying, uuid,
    character varying, character varying, character varying, character varying);
