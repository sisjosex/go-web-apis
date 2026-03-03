-- Migration: add_rider_type_column
-- Module: tracking
-- Created: 2026-03-03 15:08:50

-- Add rider_type column to riders table
ALTER TABLE tracking.riders
ADD COLUMN rider_type VARCHAR(50) DEFAULT 'employee';

-- Update rider_type to NOT NULL after setting defaults
ALTER TABLE tracking.riders
ALTER COLUMN rider_type SET NOT NULL;

-- Add constraint to enforce valid rider types
ALTER TABLE tracking.riders
ADD CONSTRAINT check_rider_type CHECK (rider_type IN ('student', 'employee'));

-- Create index for filtering by rider_type
CREATE INDEX idx_riders_type ON tracking.riders(rider_type);

-- Example stored procedure:
-- CREATE OR REPLACE FUNCTION tracking.sp_operation() RETURNS TABLE(...) AS $$
-- BEGIN
--     -- Logic here
-- END;
-- $$ LANGUAGE plpgsql;

