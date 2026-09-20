#!/bin/sh
# Installs the nightly crontab and runs crond in the foreground.
#
# crond starts its jobs with an almost empty environment, so the container's
# variables are frozen into /etc/backup.env here and sourced by backup.sh. That
# is also why `docker compose run --rm backup /backup.sh` works: the entrypoint
# is bypassed, but the script reads the live environment when the file is absent.
set -eu

# `docker compose run --rm backup /backup.sh` — an operator asking for one
# backup now. Without this the entrypoint would ignore the command and start
# the scheduler instead, and the run would hang until it was killed.
if [ "$#" -gt 0 ]; then
	exec "$@"
fi

: "${BACKUP_SCHEDULE:=0 3 * * *}"

# env -0 keeps values containing spaces or newlines intact.
env -0 | awk -v RS='\0' '
	/^(POSTGRES_|BACKUP_|AWS_|PG)/ {
		key = substr($0, 1, index($0, "=") - 1)
		val = substr($0, index($0, "=") + 1)
		gsub(/'"'"'/, "'"'"'\\'"'"''"'"'", val)
		print "export " key "='"'"'" val "'"'"'"
	}
' > /etc/backup.env
chmod 600 /etc/backup.env

echo "$BACKUP_SCHEDULE /backup.sh >> /proc/1/fd/1 2>&1" > /etc/crontabs/root

echo "🗄️  Backup scheduled: $BACKUP_SCHEDULE (keep ${BACKUP_KEEP_DAYS:-14} days)"

exec crond -f -l 8
