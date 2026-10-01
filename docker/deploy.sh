#!/bin/sh
# Run a commit on the box, from /srv/taypi24 (the git clone):
#
#   docker/deploy.sh <sha>          # a commit whose `build` workflow published the image
#   docker/deploy.sh                # the current origin/master
#
# 1. Refuse unless ghcr.io/sisjosex/api:<sha> exists; nothing has changed yet.
# 2. Check the clone out at <sha>: the compose file, Caddyfile and scripts match the image.
# 3. Pin API_TAG in .env, pull, `up -d --wait` (migrate runs before the servers).
# 4. Restart caddy only when docker/caddy changed (its files are bind-mounted one by one,
#    so a checkout leaves it reading the old inode).
#
# Rollback is the same command with the sha it prints as "was".
set -eu
cd "$(dirname "$0")/.."
COMPOSE="docker compose -f docker-compose.prod.yml"
IMAGE=ghcr.io/sisjosex/api

git fetch -q origin
SHA=$(git rev-parse --verify "${1:-origin/master}^{commit}")
WAS=$(sed -n 's/^API_TAG=//p' .env)
OLD=$(git rev-parse HEAD)

docker manifest inspect "$IMAGE:$SHA" >/dev/null 2>&1 \
	|| { echo "❌ $IMAGE:$SHA is not published — is its build workflow green?"; exit 1; }

echo "🚀 $SHA (was ${WAS:-none})"
git checkout -q --detach "$SHA"
sed -i "s/^API_TAG=.*/API_TAG=$SHA/" .env
$COMPOSE pull -q --ignore-buildable
$COMPOSE up -d --wait --wait-timeout 300
git diff --quiet "$OLD" "$SHA" -- docker/caddy || $COMPOSE restart caddy
$COMPOSE ps --format '{{.Service}}\t{{.Status}}'
echo "✅ $SHA — rollback: docker/deploy.sh ${WAS:-<previous sha>}"
