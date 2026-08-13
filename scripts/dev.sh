#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)

if [ ! -f "$project_root/.env" ]; then
  cp "$project_root/.env.example" "$project_root/.env"
  echo "Created .env from .env.example"
fi

exec docker compose -f "$project_root/compose.yaml" up --build
