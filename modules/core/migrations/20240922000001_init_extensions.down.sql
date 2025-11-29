-- Drop core extensions and schema
DROP SCHEMA IF EXISTS auth CASCADE;
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS "uuid-ossp";
