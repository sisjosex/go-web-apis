-- TRACK-074: the pre-tenancy overload of sp_list_companies, unused since the paged list replaced it,
-- read every tenant's carriers. The tenant-scoped overload stays in sql/functions/sp_list_companies.sql.
DROP FUNCTION IF EXISTS tracking.sp_list_companies(character varying, character varying);
