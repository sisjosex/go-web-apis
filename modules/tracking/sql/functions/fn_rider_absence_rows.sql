CREATE OR REPLACE FUNCTION tracking.fn_rider_absence_rows(p_tenant_id uuid, p_rider_id uuid, p_ids uuid[], p_from date, p_to date)
 RETURNS TABLE(id uuid, rider_id uuid, date_from date, date_to date, direction character varying, reason character varying, reported_by uuid, reported_by_name character varying, reported_via character varying, created_at timestamp without time zone)
 LANGUAGE sql
 STABLE
AS $function$
    SELECT
        a.id,
        a.rider_id,
        a.date_from,
        a.date_to,
        CAST(a.direction AS VARCHAR),
        CAST(a.reason AS VARCHAR),
        a.reported_by,
        CAST(COALESCE(NULLIF(CONCAT_WS(' ', u.first_name, u.last_name), ''), u.email) AS VARCHAR),
        CAST(a.reported_via AS VARCHAR),
        a.created_at
    FROM tracking.rider_absences a
    LEFT JOIN auth.users u ON u.id = a.reported_by AND u.deleted_at IS NULL
    WHERE a.tenant_id = p_tenant_id
      AND a.rider_id = p_rider_id
      AND (p_ids IS NULL OR a.id = ANY(p_ids))
      AND (p_from IS NULL OR a.date_to >= p_from)
      AND (p_to IS NULL OR a.date_from <= p_to)
    ORDER BY a.date_from, a.created_at, a.id;
$function$;
