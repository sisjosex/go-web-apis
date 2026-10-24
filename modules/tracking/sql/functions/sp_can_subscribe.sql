CREATE OR REPLACE FUNCTION tracking.sp_can_subscribe(p_tenant_id uuid, p_user_id uuid, p_guardian boolean, p_channel text, p_organization boolean DEFAULT false)
 RETURNS boolean
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_parts TEXT[] := string_to_array(p_channel, ':');
    v_uuid  CONSTANT TEXT := '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    v_id    UUID;
BEGIN
    IF array_length(v_parts, 1) NOT IN (2, 4) OR v_parts[2] !~ v_uuid THEN
        RETURN FALSE;
    END IF;
    v_id := CAST(v_parts[2] AS UUID);

    -- TRACK-067: an organization user watches its own organization's fleet channel — unless it is a
    -- supervisor with groups, whose channel would carry other groups' buses — and the trips it may see.
    IF p_organization THEN
        IF v_parts[1] = 'fleet' THEN
            RETURN array_length(v_parts, 1) = 4 AND v_id = p_tenant_id AND v_parts[3] = 'org' AND v_parts[4] ~ v_uuid
                AND EXISTS (
                    SELECT 1 FROM tracking.organization_members m
                    INNER JOIN tracking.organizations o ON o.id = m.organization_id
                    WHERE m.organization_id = CAST(v_parts[4] AS UUID)
                      AND m.user_id = p_user_id
                      AND o.tenant_id = p_tenant_id)
                AND tracking.fn_supervised_riders(p_tenant_id, p_user_id) IS NULL;
        END IF;
        RETURN v_parts[1] = 'trip' AND array_length(v_parts, 1) = 2
            AND tracking.sp_trip_visible_to_user(p_tenant_id, v_id, p_user_id);
    END IF;

    -- TRACK-081: the rail's badges, to the tenant's operators only.
    IF v_parts[1] = 'nav' THEN
        RETURN array_length(v_parts, 1) = 2 AND NOT p_guardian AND v_id = p_tenant_id;
    END IF;

    IF v_parts[1] = 'fleet' THEN
        IF p_guardian OR v_id <> p_tenant_id THEN
            RETURN FALSE;
        END IF;
        IF array_length(v_parts, 1) = 2 THEN
            RETURN TRUE;
        END IF;
        RETURN v_parts[3] = 'org' AND v_parts[4] ~ v_uuid AND EXISTS (
            SELECT 1 FROM tracking.organizations o
            WHERE o.id = CAST(v_parts[4] AS UUID) AND o.tenant_id = p_tenant_id);
    END IF;

    IF array_length(v_parts, 1) <> 2 THEN
        RETURN FALSE;
    END IF;

    IF v_parts[1] = 'driver' THEN
        RETURN EXISTS (
            SELECT 1 FROM tracking.drivers d
            WHERE d.id = v_id AND d.tenant_id = p_tenant_id AND d.user_id = p_user_id);
    END IF;

    IF v_parts[1] = 'trip' THEN
        RETURN NOT p_guardian AND EXISTS (
            SELECT 1 FROM tracking.trips t WHERE t.id = v_id AND t.tenant_id = p_tenant_id);
    END IF;

    IF v_parts[1] = 'rider' THEN
        IF NOT p_guardian THEN
            RETURN EXISTS (
                SELECT 1 FROM tracking.riders rd
                INNER JOIN tracking.organizations o ON o.id = rd.organization_id
                WHERE rd.id = v_id AND o.tenant_id = p_tenant_id);
        END IF;
        RETURN v_id = ANY(tracking.fn_guardian_scope(p_tenant_id, p_user_id)) AND EXISTS (
            SELECT 1
            FROM tracking.trips t
            INNER JOIN tracking.trip_stops ts ON ts.trip_id = t.id
            INNER JOIN tracking.trip_stop_tasks k ON k.trip_stop_id = ts.id
            WHERE t.tenant_id = p_tenant_id
              AND t.status = 'in_progress'
              AND k.subject_type = 'passenger'
              AND k.subject_id = v_id
              AND k.status <> 'cancelled');
    END IF;

    RETURN FALSE;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_can_subscribe(uuid,uuid,boolean,text,boolean) IS 'Whether the user may subscribe to a realtime channel: fleet, org fleet, trip and nav (TRACK-081) channels of the tenant for operators; a guardian only rider:{id} of their own rider while it rides a trip in progress (TRACK-025); driver:{id} only the user linked to that driver (TRACK-030); an organization user (p_organization) its organization''s fleet channel unless a supervisor with groups, and a trip carrying its riders (TRACK-067)';
