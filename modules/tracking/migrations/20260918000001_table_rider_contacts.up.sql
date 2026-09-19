-- Rider contacts: guardians and emergency contacts, tenant-scoped through the rider.
-- Replaces riders.emergency_contact (JSONB), whose values are copied here before it is dropped.

CREATE TABLE tracking.rider_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    relation VARCHAR(20) NOT NULL,
    name VARCHAR(255),
    phone VARCHAR(50),
    email VARCHAR(255),
    user_id UUID,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_rider_contact_relation CHECK (relation IN ('guardian', 'emergency', 'self'))
);

CREATE INDEX idx_rider_contacts_rider_id ON tracking.rider_contacts(rider_id);
CREATE INDEX idx_rider_contacts_user_id ON tracking.rider_contacts(user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX ux_rider_contacts_primary ON tracking.rider_contacts(rider_id, relation) WHERE is_primary;

INSERT INTO tracking.rider_contacts (rider_id, relation, name, phone, is_primary)
SELECT r.id,
       'emergency',
       NULLIF(TRIM(r.emergency_contact->>'name'), ''),
       NULLIF(TRIM(r.emergency_contact->>'phone'), ''),
       true
FROM tracking.riders r
WHERE jsonb_typeof(r.emergency_contact) = 'object'
  AND (NULLIF(TRIM(r.emergency_contact->>'name'), '') IS NOT NULL
       OR NULLIF(TRIM(r.emergency_contact->>'phone'), '') IS NOT NULL);

ALTER TABLE tracking.riders DROP COLUMN emergency_contact;
