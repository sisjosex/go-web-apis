-- Change emergency_contact column from VARCHAR to JSONB
ALTER TABLE tracking.riders 
ALTER COLUMN emergency_contact TYPE JSONB USING 
    CASE 
        WHEN emergency_contact IS NULL THEN NULL
        ELSE emergency_contact::JSONB
    END;
