# pg_dump must be at least the server's major version, so the client package
# tracks the postgres image in docker-compose.prod.yml (PostgreSQL 17). The
# client alone, not the postgres image: that one ships a whole server (~300 MB).
FROM alpine:3.22

RUN apk add --no-cache postgresql17-client aws-cli

COPY backup.sh /backup.sh
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /backup.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
