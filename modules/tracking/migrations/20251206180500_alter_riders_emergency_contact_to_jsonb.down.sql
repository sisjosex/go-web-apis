-- Revert emergency_contact column back to VARCHAR
ALTER TABLE tracking.riders 
ALTER COLUMN emergency_contact TYPE VARCHAR(255);
