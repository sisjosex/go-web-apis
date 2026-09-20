#!/bin/sh
# Runs once, on first init of an empty data directory, before init.sql.
#
# The postgres entrypoint creates exactly one database (POSTGRES_DB) and runs
# every initdb script against it. Nothing in the API ever issues CREATE
# DATABASE — the tenancy service only *migrates* a database whose URL it is
# given — so the tenant server's own DATABASE_URL database has to be created
# here or `api-tenant` starts against a database that does not exist.
#
# POSTGRES_EXTRA_DBS: space- or comma-separated database names. Empty = none.
set -e

extras=$(echo "${POSTGRES_EXTRA_DBS:-}" | tr ',' ' ')

for db in $extras; do
	[ -z "$db" ] && continue
	echo "📦 Creating extra database: $db"
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
		CREATE DATABASE "$db";
	SQL
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$db" <<-SQL
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS pgcrypto;
		CREATE SCHEMA IF NOT EXISTS auth;
	SQL
done
