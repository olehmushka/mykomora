#!/usr/bin/env bash
# Deploy one image tag to this host. Run by CI over SSH, or by hand:
#
#   deploy/deploy.sh sha-1a2b3c4          # roll forward to a tag
#   deploy/deploy.sh "$(cat deploy/.previous-tag)"   # roll back
#
# Order matters:
#   1. pull the new images (nothing running is touched),
#   2. run migrations in a one-shot container; if they fail, stop here with the
#      old version still serving,
#   3. recreate core-api and web (Caddy holds requests for a few seconds
#      meanwhile, see Caddyfile.prod),
#   4. wait for both to answer; if they do not, put the previous tag back.
#
# Migrations run BEFORE the new code starts and are never reverted by a
# rollback, so every migration must keep the previous release working
# (add columns/tables first, drop things a release later). docs/runbook.md.

set -euo pipefail

TAG="${1:?usage: deploy.sh <image-tag>}"
DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${ENV_FILE:-$DEPLOY_DIR/.env.prod}"
CURRENT_FILE="$DEPLOY_DIR/.current-tag"
PREVIOUS_FILE="$DEPLOY_DIR/.previous-tag"

compose() {
	docker compose --env-file "$ENV_FILE" -f "$DEPLOY_DIR/docker-compose.prod.yml" "$@"
}

log() { echo "$(date -u +%FT%TZ) $*"; }

healthy() {
	compose exec -T caddy wget -q -O /dev/null http://core-api:8081/healthz &&
		compose exec -T caddy wget -q -O /dev/null http://web:3000/
}

wait_healthy() {
	local i
	for i in $(seq 1 30); do
		if healthy 2>/dev/null; then return 0; fi
		sleep 2
	done
	return 1
}

current=""
[ -f "$CURRENT_FILE" ] && current="$(cat "$CURRENT_FILE")"

export IMAGE_TAG="$TAG"
log "deploying $TAG (currently ${current:-none})"

compose pull postgres caddy core-api web migrate
compose up -d postgres
compose run --rm migrate
compose up -d --remove-orphans

if wait_healthy; then
	[ -n "$current" ] && [ "$current" != "$TAG" ] && echo "$current" >"$PREVIOUS_FILE"
	echo "$TAG" >"$CURRENT_FILE"
	docker image prune -f >/dev/null
	log "deployed $TAG"
	exit 0
fi

log "ERROR: $TAG did not become healthy"
compose logs --tail=40 core-api web || true
if [ -n "$current" ] && [ "$current" != "$TAG" ]; then
	log "rolling back to $current"
	export IMAGE_TAG="$current"
	compose up -d --remove-orphans
	wait_healthy && log "rolled back to $current" || log "ERROR: rollback to $current is not healthy either"
fi
exit 1
