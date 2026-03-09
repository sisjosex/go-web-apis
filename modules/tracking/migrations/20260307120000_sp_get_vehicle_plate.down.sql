-- Rollback: sp_get_vehicle_plate
-- Module: tracking

DROP FUNCTION IF EXISTS tracking.sp_get_vehicle_plate(UUID) CASCADE;
