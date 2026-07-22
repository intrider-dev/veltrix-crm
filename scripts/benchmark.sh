#!/usr/bin/env sh
set -eu

profile="${1:-baseline}"
runs="${2:-3}"
case "$profile" in baseline|stretch) ;; *) echo "profile must be baseline or stretch" >&2; exit 2 ;; esac
case "$runs" in ''|*[!0-9]*) echo "runs must be a positive integer" >&2; exit 2 ;; esac
if [ "$runs" -lt 1 ] || [ "$runs" -gt 20 ]; then
  echo "runs must be between 1 and 20" >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
result_root="$project_root/benchmarks/results"
compose_project="veltrix-crm-benchmark"
app_port="${BENCHMARK_APP_PORT:-18080}"
postgres_port="${BENCHMARK_POSTGRES_PORT:-55433}"
stats_pid=""
mkdir -p "$result_root"

compose() {
  DEMO_SEED=false APP_PORT="$app_port" POSTGRES_PORT="$postgres_port" \
    docker compose --project-name "$compose_project" -f "$project_root/compose.yaml" "$@"
}

cleanup() {
  if [ -n "$stats_pid" ]; then
    kill "$stats_pid" >/dev/null 2>&1 || true
  fi
  compose --profile benchmark down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

wait_for_readiness() {
  attempt=1
  while [ "$attempt" -le 90 ]; do
    if curl --fail --silent "http://127.0.0.1:$app_port/api/v1/health/ready" >/dev/null; then
      return 0
    fi
    sleep 2
    attempt=$((attempt + 1))
  done
  compose logs --no-color app postgres >"$result_root/compose-readiness-failure.log" 2>&1 || true
  echo "benchmark application did not become ready" >&2
  return 1
}

docker info >/dev/null
run=1
benchmark_status=0
while [ "$run" -le "$runs" ]; do
  # This project name and its volumes are dedicated to benchmarks. Resetting it
  # prevents write traffic in one measured run from changing the next dataset.
  cleanup
  compose up -d --build postgres app
  wait_for_readiness
  compose --profile benchmark run --rm benchmark-seed

  {
    echo "date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "commit=$(git -C "$project_root" rev-parse HEAD 2>/dev/null || echo uncommitted)"
    echo "os=$(uname -a)"
    echo "cpu=$(command -v lscpu >/dev/null 2>&1 && lscpu | sed -n 's/^Model name:[[:space:]]*//p' | head -n 1 || echo unknown)"
    echo "ram_bytes=$(command -v getconf >/dev/null 2>&1 && echo $(( $(getconf _PHYS_PAGES) * $(getconf PAGE_SIZE) )) || echo unknown)"
    echo "docker=$(docker version --format '{{.Server.Version}}')"
    echo "compose=$(docker compose version --short)"
    echo "browser=not-applicable-k6"
    echo "dataset_profile=benchmark"
    echo "application_limit=0.5 CPU,128 MiB"
    echo "postgres_limit=0.5 CPU,384 MiB"
    echo "virtual_users=${CRM_BENCHMARK_VUS:-50}"
    echo "run=warm-after-${CRM_BENCHMARK_WARMUP:-1m}-warmup-and-${CRM_BENCHMARK_DURATION:-5m}-measured-on-clean-dataset"
    echo "frontend_bundle_report=benchmarks/results/bundle-report.json"
  } >"$result_root/benchmark-metadata-$profile-run-$run.txt"

  result_path="/benchmarks/results/k6-$profile-run-$run.summary.json"
  stats_path="$result_root/docker-stats-$profile-run-$run.jsonl"
  (
    while :; do
      echo "{\"sampledAt\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
      docker stats --no-stream --format '{{json .}}' \
        "$compose_project-app-1" "$compose_project-postgres-1" || true
      sleep 2
    done
  ) >"$stats_path" 2>&1 &
  stats_pid=$!
  if ! compose --profile benchmark run --rm --no-deps \
    -e "K6_PROFILE=$profile" -e "K6_RESULT_PATH=$result_path" \
    -e "CRM_BENCHMARK_VUS=${CRM_BENCHMARK_VUS:-50}" \
    -e "CRM_BENCHMARK_WARMUP=${CRM_BENCHMARK_WARMUP:-1m}" \
    -e "CRM_BENCHMARK_DURATION=${CRM_BENCHMARK_DURATION:-5m}" \
    -e "CRM_BENCHMARK_THINK_TIME=${CRM_BENCHMARK_THINK_TIME:-0.1}" benchmark \
    run --out "json=/benchmarks/results/k6-$profile-run-$run.json" \
    /benchmarks/k6/baseline.js; then
    benchmark_status=1
  fi
  kill "$stats_pid" >/dev/null 2>&1 || true
  wait "$stats_pid" 2>/dev/null || true
  stats_pid=""
  compose stats --no-stream >"$result_root/docker-stats-$profile-run-$run.txt"
  compose logs --no-color app postgres >"$result_root/compose-$profile-run-$run.log" 2>&1 || true
  run=$((run + 1))
done

node "$project_root/scripts/summarize-benchmarks.mjs" "$profile"
echo "Raw benchmark artifacts are in benchmarks/results; report medians using docs/BENCHMARK_METHODOLOGY.md."
exit "$benchmark_status"
