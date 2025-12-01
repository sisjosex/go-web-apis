-- Migration: make_tenant_id_nullable
-- Module: tracking
-- Purpose: Allow NULL tenant_id to support both multi-tenant and single-database modes
-- Created: 2025-12-01 13:10:20

-- Alter the transport_companies table to make tenant_id nullable
-- This supports super_admin creating companies in the main database without a specific tenant
ALTER TABLE tracking.transport_companies
ALTER COLUMN tenant_id DROP NOT NULL;

-- Update the comment to reflect that tenant_id can be NULL
COMMENT ON COLUMN tracking.transport_companies.tenant_id IS 
'Tenant identifier (NULL for companies in main database operated by super_admin, UUID for tenant-specific companies)';

