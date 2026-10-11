# The production PostgreSQL (INFRA-017): the image docker-compose.prod.yml ran before, plus
# pgBackRest, which archives every WAL segment to R2 and restores the cluster to any minute
# (docker/backup/README.md → Point-in-time recovery). The package comes from the PGDG apt
# repository the postgres image already has configured; that image is Debian bullseye, whose
# PGDG suite now lives on apt-archive. The base stays: another Debian is another glibc, whose
# collations the existing pgdata indexes were built under. Local development keeps the plain
# image: nothing there archives.
FROM postgis/postgis:17-3.5

RUN sed -i 's#apt.postgresql.org/pub/repos/apt#apt-archive.postgresql.org/pub/repos/apt#' /etc/apt/sources.list.d/pgdg.list \
	&& apt-get update \
	&& apt-get install -y --no-install-recommends pgbackrest \
	&& rm -rf /var/lib/apt/lists/* \
	&& install -d -o postgres -g postgres -m 750 /etc/pgbackrest /var/spool/pgbackrest /var/log/pgbackrest

COPY entrypoint-pgbackrest.sh /usr/local/bin/entrypoint-pgbackrest.sh
# Runs on first init of an empty data directory only, after the compose-mounted scripts.
COPY archive.sql /docker-entrypoint-initdb.d/30-archive.sql
RUN chmod 755 /usr/local/bin/entrypoint-pgbackrest.sh

ENTRYPOINT ["entrypoint-pgbackrest.sh"]
CMD ["postgres"]
