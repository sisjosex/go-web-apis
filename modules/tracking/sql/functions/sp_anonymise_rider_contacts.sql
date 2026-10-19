CREATE OR REPLACE FUNCTION tracking.sp_anonymise_rider_contacts(p_tenant_id uuid, p_retention_days integer)
 RETURNS TABLE(rider_id uuid, user_id uuid)
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_riders UUID[];
BEGIN
    -- The tenant's riders inactive for longer than the retention and not yet cleared (TRACK-063 D3).
    v_riders := ARRAY(
        SELECT r.id
        FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE o.tenant_id = p_tenant_id
          AND r.status = 'inactive'
          AND r.deactivated_at < now() - p_retention_days * interval '1 day'
          AND r.anonymised_at IS NULL
    );
    IF cardinality(v_riders) = 0 THEN
        RETURN;
    END IF;

    -- The contacts lose every way to reach them and their account link; the accounts unlinked here go
    -- back to the caller, which ends a membership nothing else names.
    RETURN QUERY
    WITH cleared AS (
        UPDATE tracking.rider_contacts rc
        SET name = NULL, phone = NULL, email = NULL, user_id = NULL, updated_at = CURRENT_TIMESTAMP
        FROM (SELECT c.id, c.user_id FROM tracking.rider_contacts c WHERE c.rider_id = ANY(v_riders)) old
        WHERE rc.id = old.id
        RETURNING rc.rider_id, old.user_id
    )
    SELECT DISTINCT c.rider_id, c.user_id FROM cleared c WHERE c.user_id IS NOT NULL;

    DELETE FROM tracking.rider_places p WHERE p.rider_id = ANY(v_riders);

    -- The name stays, so past trips still say who rode (D3).
    UPDATE tracking.riders r SET
        phone = NULL,
        email = NULL,
        address = NULL,
        identification_number = NULL,
        home_location = NULL,
        home_geocoded = FALSE,
        stop_place_id = NULL,
        anonymised_at = now(),
        updated_at = CURRENT_TIMESTAMP
    WHERE r.id = ANY(v_riders);
END;
$function$;

COMMENT ON FUNCTION tracking.sp_anonymise_rider_contacts(uuid,integer) IS 'Clears the contact data of the tenant''s riders inactive for more than p_retention_days — contacts'' name, phone, email and account link, the rider''s phone, email, address, document, home and saved places — keeping the name; answers the accounts it unlinked (TRACK-063 D3)';
