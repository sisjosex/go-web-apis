--
-- PostgreSQL database dump
--


SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: geo; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA geo;


--
-- Name: places; Type: TABLE; Schema: geo; Owner: -
--

CREATE TABLE geo.places (
    id bigint NOT NULL,
    name text NOT NULL,
    kind text NOT NULL,
    city text,
    search text NOT NULL,
    geom public.geography(Point,4326) NOT NULL,
    CONSTRAINT places_kind_check CHECK ((kind = ANY (ARRAY['street'::text, 'place'::text, 'poi'::text])))
);


--
-- Name: TABLE places; Type: COMMENT; Schema: geo; Owner: -
--

COMMENT ON TABLE geo.places IS 'Named streets, places and POIs of the current geo build, one point each; replaced whole by sp_replace_places (INFRA-003 D2).';


--
-- Name: places_id_seq; Type: SEQUENCE; Schema: geo; Owner: -
--

ALTER TABLE geo.places ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME geo.places_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: places_import; Type: TABLE; Schema: geo; Owner: -
--

CREATE UNLOGGED TABLE geo.places_import (
    name text,
    highway text,
    place text,
    amenity text,
    shop text,
    city text,
    geometry text
);


--
-- Name: TABLE places_import; Type: COMMENT; Schema: geo; Owner: -
--

COMMENT ON TABLE geo.places_import IS 'Staging for cli geo import: filled by COPY, emptied by sp_replace_places in the same transaction.';


--
-- Name: places places_pkey; Type: CONSTRAINT; Schema: geo; Owner: -
--

ALTER TABLE ONLY geo.places
    ADD CONSTRAINT places_pkey PRIMARY KEY (id);


--
-- Name: idx_geo_places_geom; Type: INDEX; Schema: geo; Owner: -
--

CREATE INDEX idx_geo_places_geom ON geo.places USING gist (geom);


--
-- Name: idx_geo_places_search; Type: INDEX; Schema: geo; Owner: -
--

CREATE INDEX idx_geo_places_search ON geo.places USING gin (search public.gin_trgm_ops);


--
-- PostgreSQL database dump complete
--
