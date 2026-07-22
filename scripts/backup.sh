#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
compose_file="$project_root/compose.yaml"
timestamp=$(date -u +%Y%m%dT%H%M%SZ)
output=${1:-"$project_root/backups/veltrix-crm-$timestamp.dump"}
postgres_user=${POSTGRES_USER:-postgres}
postgres_db=${POSTGRES_DB:-veltrix_crm}
container_file="/tmp/veltrix-crm-backup-$$.dump"

mkdir -p "$(dirname -- "$output")"
cleanup() {
  docker compose -f "$compose_file" exec -T postgres rm -f "$container_file" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker compose -f "$compose_file" exec -T postgres \
  pg_dump -U "$postgres_user" -d "$postgres_db" --format=custom \
  --no-owner --no-privileges --file="$container_file"
docker compose -f "$compose_file" cp "postgres:$container_file" "$output"
echo "Backup written to $output"
