CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Which SP costs what (INFRA-004): the view is read from this database, the counters
-- cover every database in the cluster. Preloaded by the compose command line; an
-- existing volume never re-runs this file, so there it is one CREATE EXTENSION by hand.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

CREATE SCHEMA IF NOT EXISTS auth;