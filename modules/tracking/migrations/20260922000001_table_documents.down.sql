-- Reverse of TRACK-016 step 1. Documents first: the FK to document_types is what holds that table.
DROP TABLE IF EXISTS tracking.compliance_documents;
DROP TABLE IF EXISTS tracking.document_types;
