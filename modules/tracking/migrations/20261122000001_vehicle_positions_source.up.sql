-- TRACK-068 D2: which receiver wrote a point, the driver's phone or a tracker bolted to the bus. The
-- phone wins while it is alive; the device fills in when it goes quiet.

ALTER TABLE tracking.vehicle_positions
    ADD COLUMN source VARCHAR(10) NOT NULL DEFAULT 'phone'
    CONSTRAINT chk_vehicle_positions_source CHECK (source IN ('phone', 'device'));

ALTER TABLE tracking.vehicle_last_position
    ADD COLUMN source VARCHAR(10) NOT NULL DEFAULT 'phone'
    CONSTRAINT chk_vehicle_last_position_source CHECK (source IN ('phone', 'device'));

COMMENT ON COLUMN tracking.vehicle_last_position.source IS
'phone: the driver app · device: a hardware tracker (OsmAnd protocol); a device point is dropped during a trip while the phone reported within GPS_PHONE_PRIORITY_S (TRACK-068 D2)';
