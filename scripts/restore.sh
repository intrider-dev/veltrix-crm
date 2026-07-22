#!/usr/bin/env sh
set -eu

if [ "$#" -ne 2 ] || [ "$2" != "RESTORE" ]; then
  echo "Usage: scripts/restore.sh <backup.dump> RESTORE" >&2
  echo "This replaces objects in the configured PostgreSQL database." >&2
  exit 2
fi

backup=$1
if [ ! -f "$backup" ]; then
  echo "Backup does not exist: $backup" >&2
  exit 2
fi
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
compose_file="$project_root/compose.yaml"
postgres_user=${POSTGRES_USER:-postgres}
postgres_db=${POSTGRES_DB:-veltrix_crm}
container_file="/tmp/veltrix-crm-restore-$$.dump"

cleanup() {
  docker compose -f "$compose_file" exec -T postgres rm -f "$container_file" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker compose -f "$compose_file" cp "$backup" "postgres:$container_file"
docker compose -f "$compose_file" exec -T postgres \
  pg_restore -U "$postgres_user" -d "$postgres_db" --clean --if-exists \
  --no-owner --no-privileges --exit-on-error "$container_file"
echo "Restore completed for database $postgres_db"
