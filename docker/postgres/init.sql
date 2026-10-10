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

-- A transaction left open and idle (a bug, a hung call inside it) holds row locks, a pool
-- connection and autovacuum's horizon; 60 s ends it. The outbox relay, the only code that waits
-- on something else mid-transaction, re-enqueues an aborted batch under the same TaskID.
-- track_io_timing adds disk-wait time to pg_stat_statements (pg_test_timing: 42 ns per read).
ALTER SYSTEM SET idle_in_transaction_session_timeout = '60s';
ALTER SYSTEM SET track_io_timing = on;

CREATE SCHEMA IF NOT EXISTS auth;