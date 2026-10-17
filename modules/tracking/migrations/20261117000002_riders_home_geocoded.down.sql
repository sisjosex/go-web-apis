-- TRACK-072 down: no geocoded flag.

ALTER TABLE tracking.riders DROP COLUMN home_geocoded;
