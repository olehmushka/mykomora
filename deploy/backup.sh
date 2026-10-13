#!/usr/bin/env bash
# Nightly Postgres backup: pg_dump (custom format) -> R2 backup bucket.
#
# Retention is NOT done here: set an R2 lifecycle rule on the backup bucket that
# deletes objects after 14 days (docs/runbook.md, "Backups"). Photos and
# attachments already live in R2 and are not copied by this script.
#
# Cron, on the VPS (as the user that runs docker):
#   17 3 * * * /opt/mykomora/deploy/backup.sh >> /var/log/mykomora-backup.log 2>&1

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${ENV_FILE:-$DEPLOY_DIR/.env.prod}"
COMPOSE=(docker compose --env-file "$ENV_FILE" -f "$DEPLOY_DIR/docker-compose.prod.yml")

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="mykomora-${stamp}.dump"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "$(date -u +%FT%TZ) dumping ${POSTGRES_DB}"
"${COMPOSE[@]}" exec -T postgres pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc >"$tmp/$file"

# A dump that cannot be listed is not a backup.
"${COMPOSE[@]}" exec -T postgres pg_restore -l <"$tmp/$file" >/dev/null

echo "$(date -u +%FT%TZ) uploading $(du -h "$tmp/$file" | cut -f1) to ${BACKUP_BUCKET}"
docker run --rm \
	-v "$tmp:/backup:ro" \
	-e AWS_ACCESS_KEY_ID="$S3_ACCESS_KEY_ID" \
	-e AWS_SECRET_ACCESS_KEY="$S3_SECRET_ACCESS_KEY" \
	-e AWS_DEFAULT_REGION="${S3_REGION:-auto}" \
	amazon/aws-cli:2 --endpoint-url "$S3_ENDPOINT" \
	s3 cp "/backup/$file" "s3://${BACKUP_BUCKET}/postgres/$file" --only-show-errors

echo "$(date -u +%FT%TZ) done: postgres/$file"
