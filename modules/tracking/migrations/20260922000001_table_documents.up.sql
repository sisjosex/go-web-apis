-- TRACK-016 step 1: the compliance documents a vehicle or a driver must carry — insurance, technical
-- inspection, a licence scan — and the types that say what "must carry" means for this tenant.
--
-- Two tables because the rule and the paper are different things: `document_types` is the tenant's
-- policy (how many days before expiry to warn, whether an expired one stops the vehicle or driver
-- working), `compliance_documents` is one piece of paper against one subject. Changing the policy then
-- changes every document at once, which is the whole point of having it stored rather than typed in.
--
-- Nothing is seeded. A type belongs to a tenant, and in single-database mode there is no tenant_id to
-- seed one for; an operator creates the handful their country requires.
--
-- `subject_type` + `subject_id` rather than two nullable FKs: a document points at exactly one thing,
-- and a pair of columns that must never both be set is a constraint waiting to be forgotten. The price
-- is no FK to the subject, which the delete SPs of drivers and vehicles answer for instead.

CREATE TABLE tracking.document_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    applies_to VARCHAR(20) NOT NULL DEFAULT 'both',
    warn_days_before INT NOT NULL DEFAULT 30,
    blocks_service BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_document_type_applies_to CHECK (applies_to IN ('vehicle', 'driver', 'both')),
    CONSTRAINT chk_document_type_warn_days CHECK (warn_days_before BETWEEN 0 AND 365),
    CONSTRAINT uk_document_type_code UNIQUE (tenant_id, code)
);

CREATE TABLE tracking.compliance_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    subject_type VARCHAR(20) NOT NULL,
    subject_id UUID NOT NULL,
    -- RESTRICT, not CASCADE: deleting a type the tenant still files documents under would take the
    -- documents with it. The types screen deactivates instead, which is why there is no delete there.
    document_type_id UUID NOT NULL REFERENCES tracking.document_types(id) ON DELETE RESTRICT,
    number VARCHAR(100),
    issued_on DATE,
    expires_on DATE,
    -- The stored file is named after the document's id (D3); these three are what the browser is told
    -- on download, so the original filename never reaches the filesystem or a URL.
    file_name VARCHAR(255),
    file_size BIGINT,
    file_ext VARCHAR(10),
    notes TEXT,
    -- When the expiry digest last named this document. NULL means "not yet warned"; editing
    -- expires_on clears it, so a renewed document warns again on its own schedule (D1).
    warned_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_document_subject_type CHECK (subject_type IN ('vehicle', 'driver'))
);

-- The one index three readers share: a subject's own documents (the detail panel), the service-blocked
-- probe per list row (D2), and the expiry page, which scans it in expires_on order.
CREATE INDEX idx_documents_subject
    ON tracking.compliance_documents (tenant_id, subject_type, subject_id, expires_on);

-- The digest and the "expiring within N days" filter read by date across the whole tenant, which the
-- index above cannot serve — its first useful column is the subject.
CREATE INDEX idx_documents_expires_on ON tracking.compliance_documents (tenant_id, expires_on);

COMMENT ON TABLE tracking.document_types IS
'A tenant''s compliance policy: what documents a vehicle or driver must carry, how early to warn before one expires, and whether an expired one stops them working (TRACK-016)';

COMMENT ON TABLE tracking.compliance_documents IS
'One document against one vehicle or driver; subject_type says which table subject_id points at, and warned_at is the expiry digest''s "already said so" mark (TRACK-016 D1)';
