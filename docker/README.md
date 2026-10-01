# The server — one box, sized (INFRA-004)

Production is one VPS (Contabo, 6 vCPU / 12 GB / NVMe, Debian 13) running
`docker-compose.prod.yml`. This is what the numbers in that file mean, how far they
go, and what the host itself needs. Backups: `backup/README.md`. Geo data: `geo/README.md`.

## Why Docker and not native packages

Containers are cgroups on the same kernel: a Go binary or PostgreSQL inside one runs
the same syscalls as outside, a named volume is a plain ext4 directory (no overlay on
`pgdata`), and traffic between containers crosses a bridge without NAT. What Docker
costs is the daemon (~70 MB) and DNAT on Caddy's 80/443 — nothing measurable next to a
query. What it gives is the ordering `migrate → api-*`, healthchecks, restarts, one
image for five roles, and Valhalla, which has no Debian package.

## Memory — 12 GB

| Service | Limit | Inside | Notes |
|---|---|---|---|
| postgres | none | `shared_buffers` 2 GB + ~1 GB connection work | `effective_cache_size` 6 GB = its buffers + the kernel cache below |
| valhalla | 1 GB | — | routing tiles, memory-mapped |
| valkey | 640 MB | `maxmemory` 512 MB, `noeviction` | full → writes refused, GPS ingest stores inline, `/readyz` degraded |
| api-tenant | 768 MB | `GOMEMLIMIT` 640 MiB | |
| api-worker | 768 MB | `GOMEMLIMIT` 640 MiB | GPS consumer, outbox relay, asynq handlers |
| api-platform | 512 MB | `GOMEMLIMIT` 420 MiB | |
| api-realtime | 512 MB | `GOMEMLIMIT` 420 MiB | one goroutine pair per socket |
| api-scheduler | 256 MB | `GOMEMLIMIT` 200 MiB | |
| caddy | 256 MB | — | |
| **kernel page cache** | **~4 GB** | | what PostgreSQL reads when a page is not in its buffers |

`GOMEMLIMIT` makes the Go collector work harder as the heap nears the limit instead of
letting the kernel kill the process; under no pressure it costs nothing. A process
killed by its limit shows `OOMKilled: true` in `docker inspect` — raise that one, not all.

## CPU — 6 vCPU

No quotas: a quota leaves cores idle, and Go 1.25 already spreads each process across
all six. `cpu_shares` only decide who wins when everyone wants CPU at once: PostgreSQL
2048, the APIs and the worker 1024, everything else 512.

## Connections — the budget behind `max_connections=200`

Every process opens its own pool per database: `DATABASE_POOL_SIZE` to its own,
`TENANCY_DATABASE_POOL_SIZE` to each tenant database it touches, plus the worker's one
`LISTEN` connection per tenant. Idle connections close after 5 min, so a tenant nobody
uses costs nothing.

| Service | own | per tenant |
|---|---|---|
| api-platform | 12 | 4 |
| api-tenant | 8 | 8 |
| api-realtime | 2 | 3 |
| api-worker | 4 | 6 + 1 |
| api-scheduler | 2 | 2 |
| **ceiling** | **28** | **+ 24 × tenants** |

28 + 24 × 7 = 196: seven tenants busy at the same moment fit. Past that, either raise
`max_connections` (each connection is ~5-10 MB when active — take it from the page cache
line above) or put PgBouncer in front (INFRA-005; it costs the relay its `LISTEN`).

## PostgreSQL

The settings are on the `postgres` command line in the compose file: NVMe costs
(`random_page_cost` 1.1, `effective_io_concurrency` 200), a checkpoint every 2 GB of WAL,
compressed WAL, `jit=off` (short SP calls lose to JIT's warm-up), two parallel workers
per query, four in total.

- Slow queries: anything over 500 ms is one line in `docker compose logs postgres`.
- Profile: `pg_stat_statements` is preloaded and created by `postgres/init.sql` on the
  first boot. On a volume that existed before INFRA-004 run it once by hand:
  `docker compose exec postgres psql -U postgres -d web -c 'CREATE EXTENSION pg_stat_statements'`.
  Then `SELECT calls, round(mean_exec_time) ms, left(query, 80) FROM pg_stat_statements
  ORDER BY total_exec_time DESC LIMIT 10;`

## Logs

Every container writes through Docker's `json-file` driver capped at 5 × 20 MB,
compressed — ~100 MB per service at most, nothing to rotate, `docker logs` unchanged.
The logs of a container are gone once it is recreated by a deploy.

- `LOG_LEVEL` in the compose `.env` (default `info`) applies to every `api-*`. `debug`
  adds one line per socket open/close, outbox purge, partition run and translation file.
- `LOG_FORMAT=json` in `.env.platform` / `.env.tenant` when a collector reads the output.
- Valhalla logs at `warn` from the next `geo/build.sh` build on; a build from before
  INFRA-004 still logs one line per request, capped like everything else.

## Host prep — Debian 13

```sh
# Docker CE from docker.com (trixie is supported); the daemon keeps containers up
# through its own restarts.
apt-get install -y ca-certificates curl && install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian trixie stable" > /etc/apt/sources.list.d/docker.list
apt-get update && apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
printf '{ "live-restore": true }\n' > /etc/docker/daemon.json && systemctl restart docker

# SSH by key only: the box is on the internet and root passwords get guessed.
printf 'PasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin prohibit-password\n' \
  > /etc/ssh/sshd_config.d/90-keys-only.conf && systemctl reload ssh

# 2 GB of swap as the safety margin for peaks (Valhalla, a vacuum), rarely touched.
fallocate -l 2G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab

# Kernel: swap late, let Valkey fork for its AOF rewrite, a deeper accept queue.
cat > /etc/sysctl.d/90-xanthops.conf <<'EOF'
vm.swappiness=10
vm.overcommit_memory=1
net.core.somaxconn=1024
EOF
sysctl --system

# Firewall: only SSH and Caddy. Docker manages the forwarding rules for its own
# published ports (80/443 are the only ones), so this covers the host itself. It
# resets only its own table: `flush ruleset` would also wipe Docker's NAT rules
# and cut every container off until the daemon restarts.
apt-get install -y nftables && cat > /etc/nftables.conf <<'EOF'
table inet filter
delete table inet filter
table inet filter {
  chain input {
    type filter hook input priority 0; policy drop;
    iif lo accept
    ct state established,related accept
    ip protocol icmp accept
    ip6 nexthdr icmpv6 accept
    tcp dport { 22, 80, 443 } accept
    udp dport 443 accept
  }
}
EOF
systemctl enable --now nftables

# Security updates for the host on their own; the pinned images weekly.
apt-get install -y unattended-upgrades && dpkg-reconfigure -plow unattended-upgrades
echo '0 4 * * 0 cd /srv/taypi24 && docker compose -f docker-compose.prod.yml pull --ignore-buildable -q && docker compose -f docker-compose.prod.yml up -d' > /etc/cron.d/xanthops-images
```

## Deploy

The API image is built by CI (`.github/workflows/build.yml`) on every push to `master`
and published as `ghcr.io/sisjosex/api:<commit sha>` and `:latest`. The package is
private like the repo, so the box logs in once with a classic token that has only
`read:packages`:

```sh
echo "$GHCR_TOKEN" | docker login ghcr.io -u sisjosex --password-stdin
```

A deploy checks the clone out at a commit, pins it in `.env` and pulls; nothing compiles on
the box:

```sh
cd /srv/taypi24 && sh docker/deploy.sh <commit sha>   # no sha: the current origin/master
```

It refuses a sha whose image is not published, waits for every service `healthy`, restarts
Caddy when `docker/caddy` changed, and prints the previous sha: rollback is the same command
with it. The checkout brings the compose file, the Caddyfile and the scripts; the image carries
the code. `docker stats` shows each service under its limit.

## First deploy (INFRA-006)

From an empty Contabo box to `https://api.taypi24.com`, in order. Every step is a command; a step
that needs something by hand on the box is a gap in this list — fix the list, not the box.

**1. DNS** (Cloudflare, zone `taypi24.com`). Records in grey cloud (DNS only): Caddy needs the
direct connection to obtain its certificates.

| Type | Name | Value |
|---|---|---|
| A | `api` | the box's IPv4 |
| A | `tiles` | the box's IPv4 |
| TXT | `@` | the mail provider's SPF (`v=spf1 include:… ~all`) |
| CNAME / TXT | the provider's | its DKIM record |

Without SPF and DKIM the password-reset mail lands in spam. `dig +short api.taypi24.com` must answer
the box before step 7, or Let's Encrypt refuses the certificate.

**2. Host prep** — the block above, as root, with your SSH public key already in
`/root/.ssh/authorized_keys` (Contabo's panel or `ssh-copy-id`). Then log in again in a second
terminal before closing the first: password logins are off from now on.

**3. Code.** A read-only deploy key, so the box can pull and never push:

```sh
ssh-keygen -t ed25519 -N '' -f /root/.ssh/deploy_api && cat /root/.ssh/deploy_api.pub
# GitHub → sisjosex/api → Settings → Deploy keys → Add (read-only), paste the line above
printf 'Host github.com\n  IdentityFile /root/.ssh/deploy_api\n' >> /root/.ssh/config
git clone git@github.com:sisjosex/api.git /srv/taypi24 && cd /srv/taypi24
```

**4. Env files.** `.env` (compose) and `.env.platform` (the servers), from the DEPLOY section of
`.env.example`; nothing else is needed (no `.env.tenant` without the tenant profile).

```sh
openssl rand -base64 32   # once each: POSTGRES_PASSWORD, VALKEY_PASSWORD, JWT_SECRET_KEY, JWT_REFRESH_KEY
```

- `.env`: `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB=web`, `VALKEY_PASSWORD`,
  `PLATFORM_HOST=api.taypi24.com`, `TILES_HOST=tiles.taypi24.com`, `ACME_EMAIL`, `API_TAG=<sha>`,
  `GEO_DATA_DIR=/srv/geo`, and the backup block of step 11.
- `.env.platform`: the "on the box" block of `.env.example` — `DATABASE_URL` and `REDIS_URL` at the
  `postgres` and `valkey` services with the passwords above, the JWT pair, `SMTP_ENABLED=true` and
  `SMTP_*` from the mail provider (without `SMTP_ENABLED` every reset, verification and OTP email
  fails with 503), `FCM_PROJECT_ID`.
- `chmod 600 .env .env.platform`.

**5. FCM key.** The Firebase service-account JSON (project `xanthops-push`), copied from the PC:

```sh
# on the PC
scp ~/.secrets/fcm-xanthops.json root@<box>:/srv/taypi24/secrets/fcm.json
# on the box
chmod 600 /srv/taypi24/secrets/fcm.json
```

Create `secrets/` first (`mkdir -m 700 /srv/taypi24/secrets`): if the file is missing at `up`, Docker
mounts an empty directory in its place and pushes stay off.

**6. Registry.** `echo "$GHCR_TOKEN" | docker login ghcr.io -u sisjosex --password-stdin` — a classic
token with `read:packages` only (Deploy, below). `API_TAG` must be a sha whose `build` workflow on
`sisjosex/api` is green.

**7. Up.**

```sh
docker compose -f docker-compose.prod.yml pull --ignore-buildable
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml ps   # migrate exited 0, the rest healthy but valhalla
```

`valhalla` restarts until step 8 gives it a build; `/geo/eta` falls back to a straight line meanwhile.

**8. Geo data.** Built on the PC, never on the box (`geo/README.md`). After step 7, because the
places import needs the schema `migrate` created:

```sh
# on the PC, from api/
GEO_REGIONS=south-america/bolivia TILES_URL=https://tiles.taypi24.com docker/geo/build.sh <date>
rsync -a docker/geo/data/<date> root@<box>:/srv/geo/
# on the box, from /srv/taypi24 — checks the build, makes it current, imports the places
GEO_DATA_DIR=/srv/geo docker/geo/switch.sh <date>
docker compose -f docker-compose.prod.yml ps   # now every service healthy
```

**9. First admin.** Register through the API, then promote the account in the database:

```sh
curl -sS https://api.taypi24.com/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"first_name":"…","last_name":"…","email":"<you>","password":"…"}'
docker compose -f docker-compose.prod.yml exec postgres sh -c \
  "psql -U \"\$POSTGRES_USER\" -d web -c \"UPDATE auth.users SET system_role='super_admin' WHERE email='<you>'\""
```

**10. Tenants** (INFRA-006 D2). A business created through self-service lives in the shared `web`
database and needs nothing here. A business on an upgraded plan gets its own database, by hand:

```sh
docker compose -f docker-compose.prod.yml exec postgres sh -c 'createdb -U "$POSTGRES_USER" <slug>'
# as the admin: POST /api/v1/tenants {"slug":"<slug>","name":"…",
#   "database_url":"postgres://<POSTGRES_USER>:<POSTGRES_PASSWORD>@postgres:5432/<slug>?sslmode=disable"}
docker compose -f docker-compose.prod.yml run --rm migrate ./cli t -migrate <slug>
```

**11. Backups to R2** (D3). Cloudflare → R2: a private bucket `taypi24-backups`, and an API token
with Object Read & Write on that bucket only. A heartbeat check (healthchecks.io's free plan) with a
one-day period and a grace of a few hours. In `.env`:

```sh
BACKUP_S3_BUCKET=taypi24-backups
BACKUP_S3_ENDPOINT=https://<account id>.r2.cloudflarestorage.com
BACKUP_S3_REGION=auto
BACKUP_S3_ACCESS_KEY=<token access key>
BACKUP_S3_SECRET_KEY=<token secret>
BACKUP_KEEP_DAYS=7
BACKUP_HEARTBEAT_URL=https://hc-ping.com/<uuid>
```

Then `docker compose -f docker-compose.prod.yml up -d backup`, one backup now
(`docker compose -f docker-compose.prod.yml run --rm backup /backup.sh`), and the scratch-database
restore check of `backup/README.md` on that dump. The heartbeat shows the ping.

**12. Off the box.** Copy `.env`, `.env.platform` and `secrets/` to the password manager or an
encrypted disk: they are the only things on the box that no backup and no repo holds.

**Check from outside** (any machine but the box):

```sh
curl -i https://api.taypi24.com/readyz                                      # 200
curl -sI -H 'Range: bytes=0-15' https://tiles.taypi24.com/basemap.pmtiles   # 206
curl -sI https://tiles.taypi24.com/style-light.json | grep -i cache-control # max-age=300
nc -zv -w 3 api.taypi24.com 5432                                            # refused / timed out
ssh -o PubkeyAuthentication=no root@api.taypi24.com                         # Permission denied
```
