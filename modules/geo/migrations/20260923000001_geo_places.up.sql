-- INFRA-003 step 3 (D2): address search without a geocoder service.
--
-- The offline build exports every named street, place and POI of the region to
-- places.geojsonl; `cli geo import` COPYs it into places_import and calls
-- sp_replace_places, one transaction, so a search never sees half an import.
-- Search is a trigram match weighed by distance, reverse is the nearest point:
-- one indexed query each, no service to run.
--
-- This lives in the tenant server's own database only — OSM is the same for
-- every tenant, so per-tenant databases never get it (tenant_service.go).

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE SCHEMA IF NOT EXISTS geo;

-- Case and accents folded, so "america" finds "Av. América". unaccent() is only
-- STABLE; naming the dictionary makes this safe to declare IMMUTABLE.
CREATE FUNCTION geo.fn_fold(p_text TEXT) RETURNS TEXT
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT lower(public.unaccent('public.unaccent'::regdictionary, p_text))
$$;

CREATE TABLE geo.places (
    id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name   TEXT NOT NULL,
    kind   TEXT NOT NULL CHECK (kind IN ('street', 'place', 'poi')),
    city   TEXT,
    search TEXT NOT NULL, -- geo.fn_fold(name), what the trigram index matches
    geom   GEOGRAPHY(Point, 4326) NOT NULL
);

CREATE INDEX idx_geo_places_search ON geo.places USING GIN (search gin_trgm_ops);
CREATE INDEX idx_geo_places_geom ON geo.places USING GIST (geom);

-- One row per exported feature, as osmium wrote it. Unlogged: it only ever
-- holds the import in flight, and is empty outside one.
CREATE UNLOGGED TABLE geo.places_import (
    name     TEXT,
    highway  TEXT,
    place    TEXT,
    amenity  TEXT,
    shop     TEXT,
    city     TEXT, -- addr:city, when the feature carries one
    geometry TEXT  -- GeoJSON geometry: Point, LineString, Polygon or MultiPolygon
);

-- Replaces geo.places with what places_import holds and empties it; returns the
-- row count. Each feature becomes one point on it. A street is many OSM ways
-- with one name, so a name keeps one point per ~1 km cell — enough to find it,
-- small enough not to flood the results. A row without a city takes the
-- nearest city or town within 30 km.
CREATE FUNCTION geo.sp_replace_places() RETURNS BIGINT
LANGUAGE plpgsql AS $$
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
$$;

-- Address search: the rows whose name holds the words of p_q, best first. Near
-- p_lat/p_lng a match outranks a better one far away (score halves at 20 km),
-- so "aroma" in Cochabamba is the avenue there before the province in La Paz.
-- No point: text match alone. A name repeats along a street (five bus stops
-- called "Aroma", one avenue in two cells), so each name, kind and city is
-- listed once, at its best-scoring point. p_limit is capped by the caller.
CREATE FUNCTION geo.sp_search_places(p_q TEXT, p_lat FLOAT8, p_lng FLOAT8, p_limit INT)
RETURNS TABLE(name TEXT, kind TEXT, city TEXT, lat FLOAT8, lng FLOAT8)
LANGUAGE sql STABLE AS $$
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
$$;

-- Reverse lookup: the nearest place within 500 m of the point, or no row.
CREATE FUNCTION geo.sp_reverse_place(p_lat FLOAT8, p_lng FLOAT8)
RETURNS TABLE(name TEXT, kind TEXT, city TEXT, lat FLOAT8, lng FLOAT8)
LANGUAGE sql STABLE AS $$
    WITH q AS (SELECT ST_SetSRID(ST_MakePoint(p_lng, p_lat), 4326)::geography AS here)
    SELECT p.name, p.kind, p.city, ST_Y(p.geom::geometry), ST_X(p.geom::geometry)
    FROM geo.places p, q
    WHERE ST_DWithin(p.geom, q.here, 500)
    ORDER BY p.geom <-> q.here
    LIMIT 1
$$;

COMMENT ON TABLE geo.places IS
    'Named streets, places and POIs of the current geo build, one point each; replaced whole by sp_replace_places (INFRA-003 D2).';
COMMENT ON TABLE geo.places_import IS
    'Staging for cli geo import: filled by COPY, emptied by sp_replace_places in the same transaction.';
COMMENT ON FUNCTION geo.fn_fold(TEXT) IS
    'Lowercase, accents removed: what geo.places.search holds and every search is folded to.';
COMMENT ON FUNCTION geo.sp_replace_places() IS
    'Replaces geo.places with geo.places_import (one point per feature, streets one per ~1 km cell, city from the nearest town) and returns the row count.';
COMMENT ON FUNCTION geo.sp_search_places(TEXT, FLOAT8, FLOAT8, INT) IS
    'Address search: trigram word match weighed by distance to p_lat/p_lng, one row per name, kind and city.';
COMMENT ON FUNCTION geo.sp_reverse_place(FLOAT8, FLOAT8) IS
    'The nearest place within 500 m of the point, or no row.';
