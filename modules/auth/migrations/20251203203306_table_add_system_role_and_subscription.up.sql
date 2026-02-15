-- Migration: table_add_system_role_and_subscription
-- Module: auth
-- Created: 2025-12-03 20:33:06

-- Add system_role column to auth.users table
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'auth' 
        AND table_name = 'users' 
        AND column_name = 'system_role'
    ) THEN
        ALTER TABLE auth.users
        ADD COLUMN system_role VARCHAR(20) DEFAULT 'user' CHECK (system_role IN ('super_admin', 'admin', 'user'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_system_role ON auth.users (system_role);

-- Add subscription_plan column for tenant limits
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'auth' 
        AND table_name = 'users' 
        AND column_name = 'subscription_plan'
    ) THEN
        ALTER TABLE auth.users
        ADD COLUMN subscription_plan VARCHAR(20) DEFAULT 'free' CHECK (subscription_plan IN ('free', 'pro', 'enterprise'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_subscription_plan ON auth.users (subscription_plan);

COMMENT ON COLUMN auth.users.system_role IS 'System-level role: super_admin (full access), admin (user management), user (default)';
COMMENT ON COLUMN auth.users.subscription_plan IS 'Subscription plan affecting tenant limits: free (1 tenant), pro (5 tenants), enterprise (unlimited)';

