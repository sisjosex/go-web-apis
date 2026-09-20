# Backup and restore

Nightly logical backup of the whole cluster to S3-compatible storage. One
`crond` in the `backup` container runs `/backup.sh` on `BACKUP_SCHEDULE`.

## What lands in the bucket

```
s3://$BACKUP_S3_BUCKET/
  2026-09-19/
    globals.sql      # pg_dumpall --globals-only: roles, tablespaces
    web.dump         # pg_dump -Fc, one per database in pg_database
    tenant_db.dump
    acme.dump        # tenants that appeared after the container started
```

Databases are enumerated at run time, so a tenant database created today is in
tonight's backup with no redeploy. Anything older than `BACKUP_KEEP_DAYS` days
is pruned at the end of each run.

## Variables

| Variable | Default | Meaning |
|---|---|---|
| `BACKUP_SCHEDULE` | `0 3 * * *` | crontab line, UTC |
| `BACKUP_KEEP_DAYS` | `14` | prune date prefixes older than this |
| `BACKUP_S3_BUCKET` | — | required |
| `BACKUP_S3_ENDPOINT` | — | MinIO / R2 / Spaces / Backblaze; empty = real AWS S3 |
| `BACKUP_S3_ACCESS_KEY` / `BACKUP_S3_SECRET_KEY` | — | credentials |
| `BACKUP_S3_REGION` | `us-east-1` | |
| `POSTGRES_HOST` / `POSTGRES_PORT` | `postgres` / `5432` | |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` | — | must be a superuser for `--globals-only` |

## Run one now, without waiting for the schedule

```bash
docker compose -f docker-compose.prod.yml run --rm backup /backup.sh
```

## Restore

Dumps are custom-format, so `pg_restore`, not `psql`. Fetch the day you want:

```bash
aws s3 cp s3://$BACKUP_S3_BUCKET/2026-09-19/ . --recursive \
  --endpoint-url $BACKUP_S3_ENDPOINT
```

**1. Roles first** — a dump references role names but does not create them. Skip
this if the cluster already has its roles:

```bash
psql -h <host> -U postgres -d postgres -f globals.sql
```

**2. One database.** `--clean --if-exists` drops each object before recreating
it, so the target may already hold an older copy:

```bash
pg_restore -h <host> -U postgres -d web --clean --if-exists web.dump
```

The target database must exist (`createdb -h <host> -U postgres web`), because a
custom-format dump of one database contains no `CREATE DATABASE`.

**3. Into a scratch database instead** — the safe way to check a dump without
touching production:

```bash
createdb -h <host> -U postgres restore_check
pg_restore -h <host> -U postgres -d restore_check web.dump
psql -h <host> -U postgres -d restore_check -Atc \
  "SELECT count(*) FROM pg_tables WHERE schemaname NOT IN ('pg_catalog','information_schema')"
```

That count must match the source. `pg_restore` warns about the `postgres` role
and about extensions it cannot recreate as a non-superuser; those warnings are
expected and do not mean the restore failed — a non-zero exit code does.

## What this is not

Point-in-time recovery. A nightly dump loses up to 24 hours. PITR (pgBackRest or
WAL-G) and a scheduled restore test are INFRA-004.
