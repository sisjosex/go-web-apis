-- TRACK-072 D3: a rider whose home the import placed by its address, until an operator confirms the pin.

ALTER TABLE tracking.riders ADD COLUMN home_geocoded BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN tracking.riders.home_geocoded IS
'The home pin came from geocoding the address at import, not from a person; moving the pin clears it (TRACK-072 D3)';
