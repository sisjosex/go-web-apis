-- TRACK-005 step 1 (D2): `company_client_access` granted a whole other tenant read access to a
-- carrier, which is the cross-tenant read ARCH-001 N1 rules out. Organizations and their members
-- replace it: a client is an organization inside this tenant, and the people who may see it are its
-- members. The grants held only development data, so nothing is carried over.

DROP FUNCTION IF EXISTS tracking.sp_grant_client_access(UUID, UUID, UUID, VARCHAR, UUID, TEXT);
DROP FUNCTION IF EXISTS tracking.sp_revoke_client_access(UUID, UUID, UUID, UUID);
DROP FUNCTION IF EXISTS tracking.sp_list_company_clients(UUID, UUID);

DROP TABLE IF EXISTS tracking.company_client_access;
