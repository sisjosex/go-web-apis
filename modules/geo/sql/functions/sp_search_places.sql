CREATE OR REPLACE FUNCTION geo.sp_search_places(p_q text, p_lat double precision, p_lng double precision, p_limit integer)
 RETURNS TABLE(name text, kind text, city text, lat double precision, lng double precision)
 LANGUAGE sql
 STABLE
AS $function$
    WITH q AS (
        SELECT geo.fn_fold(p_q) AS s,
               CASE WHEN p_lat IS NOT NULL AND p_lng IS NOT NULL
                    THEN ST_SetSRID(ST_MakePoint(p_lng, p_lat), 4326)::geography END AS here
    ),
    scored AS (
        SELECT DISTINCT ON (p.search, p.kind, p.city)
               p.name, p.kind, p.city, p.geom,
               word_similarity(q.s, p.search)
                   / (1 + coalesce(ST_Distance(p.geom, q.here), 0) / 20000) AS score,
               similarity(q.s, p.search) AS whole
        FROM geo.places p, q
        WHERE q.s <% p.search
        ORDER BY p.search, p.kind, p.city, score DESC
    )
    SELECT s.name, s.kind, s.city, ST_Y(s.geom::geometry), ST_X(s.geom::geometry)
    FROM scored s
    ORDER BY s.score DESC, s.whole DESC
    LIMIT p_limit
$function$;

COMMENT ON FUNCTION geo.sp_search_places(text,double precision,double precision,integer) IS 'Address search: trigram word match weighed by distance to p_lat/p_lng, one row per name, kind and city.';
