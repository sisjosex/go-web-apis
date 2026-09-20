# pg_dump must be at least the server's major version, so this tracks the
# postgres image in docker-compose.prod.yml (PostgreSQL 17).
FROM postgres:17-alpine

RUN apk add --no-cache aws-cli

COPY backup.sh /backup.sh
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /backup.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
