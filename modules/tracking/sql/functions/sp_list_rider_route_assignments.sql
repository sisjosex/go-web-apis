CREATE OR REPLACE FUNCTION tracking.sp_list_rider_route_assignments(p_tenant_id uuid, p_rider_id uuid DEFAULT NULL::uuid, p_route_id uuid DEFAULT NULL::uuid, p_date date DEFAULT NULL::date, p_scope_user_id uuid DEFAULT NULL::uuid, p_page integer DEFAULT 1, p_page_size integer DEFAULT 20)
 RETURNS TABLE(id uuid, rider_id uuid, rider_name character varying, route_id uuid, route_name character varying, direction character varying, days_of_week smallint, pickup_stop_place_id uuid, pickup_stop_name character varying, dropoff_stop_place_id uuid, dropoff_stop_name character varying, valid_from date, valid_until date, created_at timestamp without time zone, updated_at timestamp without time zone, total_count bigint)
 LANGUAGE plpgsql
 STABLE
AS $function$
DECLARE
    v_scope UUID[];
BEGIN
    IF p_scope_user_id IS NOT NULL THEN
        v_scope := tracking.fn_organization_scope(p_tenant_id, p_scope_user_id);
    END IF;

    RETURN QUERY
    SELECT a.*, CAST(COUNT(*) OVER () AS BIGINT)
    FROM tracking.fn_rider_route_assignment_rows(p_tenant_id, NULL) a
    WHERE (p_rider_id IS NULL OR a.rider_id = p_rider_id)
      AND (p_route_id IS NULL OR a.route_id = p_route_id)
      AND (p_date IS NULL OR tracking.fn_assignment_on(a.days_of_week, a.valid_from, a.valid_until, p_date))
      AND (v_scope IS NULL OR EXISTS (
          SELECT 1 FROM tracking.riders rd
          WHERE rd.id = a.rider_id AND rd.organization_id = ANY(v_scope)
      ))
    ORDER BY a.rider_name ASC, a.valid_from ASC, a.id ASC
    LIMIT  p_page_size
    OFFSET (p_page - 1) * p_page_size;
END;
$function$;

COMMENT ON FUNCTION tracking.sp_list_rider_route_assignments(uuid,uuid,uuid,date,uuid,integer,integer) IS 'One page of a tenant''s assignments with names, optionally by rider, route and a day they are in force on, narrowed to p_scope_user_id''s organizations when it is set; every row carries total_count (TRACK-009)';
