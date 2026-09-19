-- Restores riders.emergency_contact from the primary emergency contact; guardian rows are lost.
ALTER TABLE tracking.riders ADD COLUMN emergency_contact JSONB;

UPDATE tracking.riders r
SET emergency_contact = jsonb_build_object('name', c.name, 'phone', c.phone)
FROM tracking.rider_contacts c
WHERE c.rider_id = r.id AND c.relation = 'emergency' AND c.is_primary;

DROP TABLE IF EXISTS tracking.rider_contacts;
