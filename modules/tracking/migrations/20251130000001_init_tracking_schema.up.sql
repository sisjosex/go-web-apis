-- Create tracking schema
CREATE SCHEMA IF NOT EXISTS tracking;

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enable PostGIS for geographic data (optional but recommended for location tracking)
-- CREATE EXTENSION IF NOT EXISTS postgis;
