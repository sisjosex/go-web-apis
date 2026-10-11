# Backup and restore

Nightly logical backup of the whole cluster to S3-compatible storage, and the
point-in-time archive beside it (below). One
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
| `BACKUP_S3_REGION` | `us-east-1` | `auto` for R2 |
| `BACKUP_HEARTBEAT_URL` | — | one GET after a complete run; a heartbeat service alerts on a missing night |
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

## Point-in-time recovery

The dump above loses up to 24 hours. pgBackRest, inside the `postgres` image (INFRA-017),
archives every WAL segment to `s3://$BACKUP_S3_BUCKET/pgbackrest/` within a minute, on top of a
full base backup each Sunday and a differential each other night (`docker/README.md → Backups`
has the cron and the measured recovery time). It restores the **whole cluster** — every
database — to any minute of the last 8-14 days. Same bucket and token as the dump; the prune
above only touches date prefixes.

From `/srv/taypi24` on the box:

```sh
docker/restore.sh info                                     # backups, WAL range, last archived segment
docker/restore.sh drill --time '2026-10-11 14:05:00+00'    # restore into pg-drill, print tables and seconds per phase
docker/restore.sh drill                                    # same, to the last archived minute
docker/restore.sh drill clean                              # remove pg-drill and its volume
docker/restore.sh restore --time '2026-10-11 14:05:00+00' --yes   # production: stops the stack, restores pgdata, starts it
```

`drill` never touches `pgdata`: its own container and volume, archiving off, the table count
next to production's. Rehearse the exact `--time` with it before `restore`, which refuses
without `--yes`. After a real restore take a full backup (the command is printed): the
cluster continues on a new timeline.

Use the **dump** instead when one database is wrong and the others are not (a tenant with its
own database), or to move data to another PostgreSQL version. To pull one database out of a
point in time, `drill` to that minute and `pg_dump` it from `pg-drill`.
