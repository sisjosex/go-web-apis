-- Rollback: add_rider_type_column
-- Module: tracking

-- Drop indexes
DROP INDEX IF EXISTS tracking.idx_riders_type;

-- Drop constraint
ALTER TABLE tracking.riders DROP CONSTRAINT IF EXISTS check_rider_type;

-- Remove column
ALTER TABLE tracking.riders DROP COLUMN IF EXISTS rider_type;