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
# published ports (80/443 are the only ones), so this covers the host itself.
apt-get install -y nftables && cat > /etc/nftables.conf <<'EOF'
flush ruleset
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
echo '0 4 * * 0 cd /srv/xanthops && docker compose -f docker-compose.prod.yml pull --ignore-buildable -q && docker compose -f docker-compose.prod.yml up -d' > /etc/cron.d/xanthops-images
```

## Deploy

```sh
cd /srv/xanthops && git pull && docker compose -f docker-compose.prod.yml up -d --build
```

The image builds on the box: the module download layer is cached, only the compile
step reruns. `docker compose ps` shows every service `healthy`; `docker stats` shows
each one under its limit.
