CREATE OR REPLACE FUNCTION geo.sp_replace_places()
 RETURNS bigint
 LANGUAGE plpgsql
AS $function$
DECLARE
    v_count BIGINT;
BEGIN
    CREATE TEMP TABLE places_src ON COMMIT DROP AS
    SELECT btrim(i.name) AS name,
           CASE
               WHEN i.place IS NOT NULL THEN 'place'
               WHEN i.highway IS NOT NULL AND GeometryType(g.shape) <> 'POINT' THEN 'street'
               ELSE 'poi'
           END AS kind,
           i.place,
           NULLIF(btrim(i.city), '') AS city,
           -- A broken OSM polygon still has a centroid; PointOnSurface would raise.
           CASE WHEN ST_IsValid(g.shape) THEN ST_PointOnSurface(g.shape) ELSE ST_Centroid(g.shape) END AS pt
    FROM geo.places_import i
    CROSS JOIN LATERAL (SELECT ST_GeomFromGeoJSON(i.geometry) AS shape) g
    WHERE btrim(coalesce(i.name, '')) <> ''
      AND coalesce(i.highway, i.place, i.amenity, i.shop) IS NOT NULL;

    CREATE TEMP TABLE places_towns ON COMMIT DROP AS
    SELECT name, pt::geography AS geom FROM places_src WHERE place IN ('city', 'town');
    CREATE INDEX ON places_towns USING GIST (geom);

    TRUNCATE geo.places;

    INSERT INTO geo.places (name, kind, city, search, geom)
    SELECT DISTINCT ON (s.kind, geo.fn_fold(s.name),
                        CASE WHEN s.kind = 'street' THEN ST_SnapToGrid(s.pt, 0.01) ELSE s.pt END)
           s.name, s.kind, s.city, geo.fn_fold(s.name), s.pt::geography
    FROM places_src s
    WHERE s.pt IS NOT NULL;

    UPDATE geo.places p
    SET city = (
        SELECT tw.name FROM places_towns tw
        WHERE ST_DWithin(tw.geom, p.geom, 30000)
        ORDER BY tw.geom <-> p.geom
        LIMIT 1
    )
    WHERE p.city IS NULL AND p.kind <> 'place';

    SELECT count(*) INTO v_count FROM geo.places;
    TRUNCATE geo.places_import;
    ANALYZE geo.places;
    RETURN v_count;
END;
$function$;

COMMENT ON FUNCTION geo.sp_replace_places() IS 'Replaces geo.places with geo.places_import (one point per feature, streets one per ~1 km cell, city from the nearest town) and returns the row count.';
