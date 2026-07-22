#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
compose_file="$project_root/compose.yaml"

docker compose -f "$compose_file" up -d postgres
docker compose -f "$compose_file" run --rm --no-deps app migrate
