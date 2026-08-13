#!/usr/bin/env sh
set -eu

usage() {
  cat <<'EOF'
Usage:
  ./scripts/benchmark-browser.sh [--base-url URL] [--cycles 20]
    [--lighthouse-profile desktop|mobile|all] [--lighthouse-path /login]
    [--chrome-path PATH] [--headed] [--skip-measurement] [--skip-lighthouse]
    [--dry-run]

Credentials are read only from E2E_EMAIL and E2E_PASSWORD. Artifacts are written
under benchmarks/results and are ignored by Git. --chrome-path overrides local
Chrome/Chromium for Lighthouse only.
EOF
}

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
base_url="${BROWSER_BENCHMARK_BASE_URL:-${E2E_BASE_URL:-http://127.0.0.1:8080}}"
cycles=20
lighthouse_profile=all
lighthouse_path=/login
chrome_path="${LIGHTHOUSE_CHROME_PATH:-${CHROME_PATH:-}}"
headed=false
skip_measurement=false
skip_lighthouse=false
dry_run=false
node_command=node

if ! "$node_command" --version >/dev/null 2>&1; then
  if command -v node.exe >/dev/null 2>&1 && node.exe --version >/dev/null 2>&1; then
    node_command=node.exe
  else
    echo 'a working Node.js executable is required' >&2
    exit 1
  fi
fi

while [ "$#" -gt 0 ]; do
  case "$1" in
    --base-url) [ "$#" -ge 2 ] || { echo '--base-url requires a value' >&2; exit 2; }; base_url=$2; shift 2 ;;
    --cycles) [ "$#" -ge 2 ] || { echo '--cycles requires a value' >&2; exit 2; }; cycles=$2; shift 2 ;;
    --lighthouse-profile) [ "$#" -ge 2 ] || { echo '--lighthouse-profile requires a value' >&2; exit 2; }; lighthouse_profile=$2; shift 2 ;;
    --lighthouse-path) [ "$#" -ge 2 ] || { echo '--lighthouse-path requires a value' >&2; exit 2; }; lighthouse_path=$2; shift 2 ;;
    --chrome-path) [ "$#" -ge 2 ] || { echo '--chrome-path requires a value' >&2; exit 2; }; chrome_path=$2; shift 2 ;;
    --headed) headed=true; shift ;;
    --skip-measurement) skip_measurement=true; shift ;;
    --skip-lighthouse) skip_lighthouse=true; shift ;;
    --dry-run) dry_run=true; shift ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

case "$cycles" in ''|*[!0-9]*) echo 'cycles must be an integer' >&2; exit 2 ;; esac
if [ "$cycles" -lt 1 ] || [ "$cycles" -gt 100 ]; then
  echo 'cycles must be between 1 and 100' >&2
  exit 2
fi
case "$lighthouse_profile" in desktop|mobile|all) ;; *) echo 'invalid Lighthouse profile' >&2; exit 2 ;; esac

cd "$project_root"

if [ "$dry_run" = true ]; then
  if [ "$skip_measurement" = false ]; then
    headed_flag=''
    if [ "$headed" = true ]; then headed_flag=' --headed'; fi
    echo "$node_command benchmarks/browser/measure-performance.mjs --base-url $base_url --cycles $cycles$headed_flag"
  fi
  if [ "$skip_lighthouse" = false ]; then
    if [ -n "$chrome_path" ]; then
      "$node_command" scripts/run-lighthouse.mjs --base-url "$base_url" --path "$lighthouse_path" \
        --profile "$lighthouse_profile" --chrome-path "$chrome_path" --dry-run
    else
      "$node_command" scripts/run-lighthouse.mjs --base-url "$base_url" --path "$lighthouse_path" \
        --profile "$lighthouse_profile" --dry-run
    fi
  fi
  exit 0
fi

if [ "$skip_measurement" = false ]; then
  if [ "$headed" = true ]; then
    "$node_command" benchmarks/browser/measure-performance.mjs --base-url "$base_url" --cycles "$cycles" --headed
  else
    "$node_command" benchmarks/browser/measure-performance.mjs --base-url "$base_url" --cycles "$cycles"
  fi
fi

if [ "$skip_lighthouse" = false ]; then
  if [ -n "$chrome_path" ]; then
    "$node_command" scripts/run-lighthouse.mjs --base-url "$base_url" --path "$lighthouse_path" \
      --profile "$lighthouse_profile" --chrome-path "$chrome_path"
  else
    "$node_command" scripts/run-lighthouse.mjs --base-url "$base_url" --path "$lighthouse_path" \
      --profile "$lighthouse_profile"
  fi
fi
