#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
compose_file="$project_root/compose.yaml"

docker compose -f "$compose_file" up -d --no-recreate --wait --wait-timeout 120 postgres
# Administrative credentials exist only in the PostgreSQL container. Execute
# a freshly built, finite one-shot command instead of weakening or restarting
# the scratch application service or the live database process.
docker compose -f "$compose_file" build postgres
docker compose -f "$compose_file" run --rm --no-deps --user postgres \
  --entrypoint /app/veltrix-crm postgres migrate
