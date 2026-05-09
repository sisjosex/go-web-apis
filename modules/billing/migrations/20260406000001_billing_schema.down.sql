-- Rollback: billing_schema
DROP TABLE IF EXISTS billing.payments CASCADE;
DROP TABLE IF EXISTS billing.plan_limits CASCADE;
DROP TABLE IF EXISTS billing.subscriptions CASCADE;
DROP SCHEMA IF EXISTS billing CASCADE;
