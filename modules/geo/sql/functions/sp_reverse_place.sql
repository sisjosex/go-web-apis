CREATE OR REPLACE FUNCTION geo.sp_reverse_place(p_lat double precision, p_lng double precision)
 RETURNS TABLE(name text, kind text, city text, lat double precision, lng double precision)
 LANGUAGE sql
 STABLE
AS $function$
    WITH q AS (SELECT ST_SetSRID(ST_MakePoint(p_lng, p_lat), 4326)::geography AS here)
    SELECT p.name, p.kind, p.city, ST_Y(p.geom::geometry), ST_X(p.geom::geometry)
    FROM geo.places p, q
    WHERE ST_DWithin(p.geom, q.here, 500)
    ORDER BY p.geom <-> q.here
    LIMIT 1
$function$;

COMMENT ON FUNCTION geo.sp_reverse_place(double precision,double precision) IS 'The nearest place within 500 m of the point, or no row.';
