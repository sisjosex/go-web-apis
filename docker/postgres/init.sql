CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Which SP costs what (INFRA-004): the view is read from this database, the counters
-- cover every database in the cluster. Preloaded by the compose command line; an
-- existing volume never re-runs this file, so there it is one CREATE EXTENSION by hand.
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- One line per statement over 500 ms (INFRA-016). In postgresql.auto.conf, not on the
-- compose command line, so docker/observe.sh can flip it without a restart; an existing
-- volume gets it from `docker/observe.sh querylog off`.
ALTER SYSTEM SET log_min_duration_statement = '500ms';

CREATE SCHEMA IF NOT EXISTS auth;