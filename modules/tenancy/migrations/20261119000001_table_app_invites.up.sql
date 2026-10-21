-- TRACK-062 D3: an app invitation per account and business, in the main database beside the
-- membership it comes with. Written by sp_grant_app_access; accepted on the account's first sign-in
-- after it (sp_accept_app_invites), so one person invited by two businesses signs in once.
CREATE TABLE tenancy.app_invites (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES tenancy.tenants(id) ON DELETE CASCADE,
    user_id     UUID         NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    level       VARCHAR(20)  NOT NULL,
    -- How the invitation is meant to reach the person: the access email, or WhatsApp opened by the
    -- operator (D2).
    channel     VARCHAR(20)  NOT NULL,
    destination VARCHAR(255),
    created_by  UUID         REFERENCES auth.users(id) ON DELETE SET NULL,
    sent_at     TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_invites_level CHECK (level IN ('portal', 'driver', 'organization')),
    CONSTRAINT chk_app_invites_channel CHECK (channel IN ('whatsapp', 'email')),
    CONSTRAINT uq_app_invites_tenant_user UNIQUE (tenant_id, user_id)
);

-- The sign-in accept looks up the account's open invitations.
CREATE INDEX idx_app_invites_user_open ON tenancy.app_invites (user_id) WHERE accepted_at IS NULL;

COMMENT ON TABLE tenancy.app_invites IS
'An app access invitation per account and business: its channel and destination, when it went out and when the person first signed in after it (TRACK-062 D3); the status is derived, never stored';
