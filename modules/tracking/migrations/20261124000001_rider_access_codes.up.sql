-- MOBILE-042: a passenger's way into the app. One use, seven days, never the card's code: whoever sees a
-- printed card or a pass reaches nothing with it (D1). Only the hash is kept, as share links do.

CREATE TABLE tracking.rider_access_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    rider_id UUID NOT NULL REFERENCES tracking.riders(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    used_by UUID,
    revoked_at TIMESTAMPTZ,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One live code per rider: a new one revokes the last.
CREATE UNIQUE INDEX uq_rider_access_codes_live ON tracking.rider_access_codes (rider_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;

COMMENT ON TABLE tracking.rider_access_codes IS
'One-time codes the office hands a passenger to sign in by QR (MOBILE-042). Only the hash of the code is stored.';
