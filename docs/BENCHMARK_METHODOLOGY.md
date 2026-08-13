# Reproducible benchmark methodology

This protocol measures the project under a fixed, resource-constrained profile. It is designed to expose regressions, not manufacture a favorable headline.

## Rules of evidence

1. Benchmark an immutable commit and record whether the tree is clean.
2. Retain raw k6 summaries, browser traces/HAR where appropriate, bundle report, and container stats.
3. Run the baseline at least three times after a separate warm-up; publish medians and all p95/p99/error/throughput values.
4. Do not discard failed runs unless the cause is external and documented; retain both the failure and rerun.
5. Keep dataset, limits, duration, browser profile, and host power mode unchanged during comparisons.
6. Never publish the best single run as the primary value.
7. A threshold failure remains visible. Do not reduce VUs, dataset, or assertions to turn it green.

## Required metadata

Record before each report:

- date/time and commit SHA;
- clean/dirty working tree;
- OS/version, CPU model/core count, RAM, storage type;
- power plan/CPU governor and other substantial host load;
- Docker Engine and Compose versions;
- browser name/version and Playwright version;
- dataset profile and verified row counts;
- app/PostgreSQL CPU, memory, PID, pool, and worker limits;
- cold or warm scenario, warm-up/measured durations, and run count;
- network shaping if any.

Commands that help collect this metadata should be copied verbatim into the report. Redact usernames, hostnames, credentials, and tokens.

## Fixed datasets

All records are deterministic and synthetic. They must not contain real people, domains, phone numbers, notes, or customer data.

| Profile     | Contacts | Companies |  Deals | Activities |
| ----------- | -------: | --------: | -----: | ---------: |
| `small`     |    1,000 |       250 |    500 |      5,000 |
| `benchmark` |  100,000 |    25,000 | 50,000 |    500,000 |

After seeding, query and record actual tenant-scoped row counts. A partial seed invalidates a run. Each independent load run uses a fresh, benchmark-only Compose project and volume because the workload contains writes; otherwise later runs would start from a larger dataset. The measured window is still warm: it begins only after the fixed one-minute warm-up. Development volumes use another project name and are never reset by the benchmark scripts.

## Server baseline

| Parameter         | Baseline                                                  |
| ----------------- | --------------------------------------------------------- |
| Application       | 0.5 CPU, 128 MB, bounded 8-connection default pool        |
| PostgreSQL        | 0.5 CPU, 384 MB, 50 max connections, 64 MB shared buffers |
| Virtual users     | 50 concurrent                                             |
| Warm-up           | 1 minute                                                  |
| Measured duration | 5 minutes minimum                                         |
| Runs              | 3 minimum                                                 |
| Think time        | 100 ms default                                            |

The pinned k6 scenario in `benchmarks/k6/baseline.js` produces this deterministic operation distribution over every 100 iterations:

| Operation                      | Share |
| ------------------------------ | ----: |
| Contact list, 50-row page      |   45% |
| Dashboard read                 |   20% |
| Global search                  |   17% |
| Contact list + one detail read |   10% |
| Idempotent contact create      |    8% |

The first session login per VU is setup traffic and not added to the custom CRM latency trends. Errors from measured operations are always counted.

Thresholds:

- error rate `< 0.5%`;
- ordinary read p95 `< 150 ms`;
- write p95 `< 250 ms`;
- global-search p95 `< 250 ms`;
- no OOM, sustained pool starvation, runaway goroutines, or unexplained query-plan collapse;
- report p99 and operations/second even when thresholds pass.

## Stretch profile

Run the identical workload with 100 VUs after baseline. Do not call a stretch threshold miss a baseline failure. Report the degradation ratio, error/latency curve, app and database CPU/RSS, pool wait, and the first observed bottleneck. Do not tune between baseline and stretch unless both profiles are rerun and the change is recorded.

## Execution

Linux/macOS:

```bash
./scripts/benchmark.sh baseline 3
./scripts/benchmark.sh stretch 3
```

PowerShell:

```powershell
./scripts/benchmark.ps1 -Profile baseline -Runs 3
./scripts/benchmark.ps1 -Profile stretch -Runs 3
```

The scripts build/start `postgres` and `app` in the isolated `veltrix-crm-benchmark` Compose project, disable the smaller demo seed, recreate the deterministic benchmark dataset for every independent run, execute k6 in a pinned container, and save no-stream Docker stats and service logs for each run. They also generate `k6-<profile>-summary.json` with the median of per-run values. Root `pnpm benchmark` invokes the three-run PowerShell baseline.

Raw output is written under `benchmarks/results/`, which is ignored except for `.gitkeep`. CI should upload it as an artifact. A release can copy an immutable report into a versioned evidence directory after removing credentials and verifying the commit SHA.

## Aggregation

For each run, extract:

- `crm_read_latency`, `crm_write_latency`, `crm_search_latency`: median, p95, p99;
- `crm_errors`: rate and count;
- `crm_operations`: count and rate;
- HTTP request p95/p99 and checks;
- app/PostgreSQL current and peak RSS/CPU where available;
- connection/pool and OOM/restart evidence.

The report's primary central value is the median of the per-run values. Also show each run and min/max or spread; do not average percentiles into a value labeled as a percentile. If one run failed for an application reason, include it in error/failure reporting.

## Cold and warm definitions

- **Cold application load:** new browser context, empty HTTP cache/service-worker state, application already ready unless startup is the metric.
- **Cold startup:** containers not running; timer starts immediately before `compose up` and stops at the first successful readiness response.
- **Cold database cache:** new database container/volume or explicitly documented host restart; do not claim this if only the app restarted.
- **Warm:** application ready, seed complete, one unreported warm-up traversal/load completed, same fixed dataset.

Report cold and warm separately. Never mix them into a single median.

## Browser performance

Use the production-like same-origin build, Chromium version recorded by Playwright, UTC timezone, a fresh browser profile, and stable viewports. Disable extensions. Close unrelated heavy applications or record them.

Required scenarios:

1. Cold and warm application load.
2. Contacts list, search, details, create, and return to list.
3. Deal create and optimistic stage move.
4. Dashboard and reports load.
5. Ten minutes of representative navigation.
6. Twenty list → details → list cycles followed by forced GC in a browser launched with the required debugging flags.
7. Large-list scrolling trace after data is rendered.

Collect:

- Lighthouse desktop/mobile performance and accessibility;
- LCP, CLS, INP or controlled interaction latency;
- transferred bytes and request count with cache state recorded;
- DOM node count;
- used/total JS heap after stabilization;
- retained heap growth after 20 cycles;
- animation/scroll frames and long tasks.

The browser budgets and current status are in [`PERFORMANCE.md`](PERFORMANCE.md). A Playwright pass alone is not a Lighthouse or heap measurement.

### Reproducible local commands

Start the production image with the deterministic seed before collecting browser evidence. The root command runs the authenticated CDP scenario followed by pinned Lighthouse desktop and mobile audits:

```bash
E2E_BASE_URL=http://127.0.0.1:8080 pnpm benchmark:browser
```

The cross-platform wrappers expose the same workflow and permit a local Chrome override for Lighthouse:

```bash
./scripts/benchmark-browser.sh --base-url http://127.0.0.1:8080 \
  --chrome-path /path/to/chrome
```

```powershell
./scripts/benchmark-browser.ps1 -BaseUrl http://127.0.0.1:8080 `
  -ChromePath 'C:\Program Files\Google\Chrome\Application\chrome.exe'
```

Use `--dry-run`/`-DryRun` to inspect the exact Lighthouse commands without opening the application. The lower-level commands are `pnpm benchmark:browser:measure` and `pnpm benchmark:lighthouse`. Credentials are accepted only through `E2E_EMAIL` and `E2E_PASSWORD`; they are never written to an artifact.

The authenticated script writes `benchmarks/results/browser-performance.json`. It uses the repository-pinned `@playwright/test` Chromium, a fresh context, UTC, `en-US`, a fixed viewport, and blocked service workers. Before timing, it normalizes the local demo user's persisted application language to English through the same-origin CSRF-protected API and records the resulting document locale; credentials and CSRF values are never retained. After one demo login it measures authenticated document navigation to dashboard and contacts with a warm HTTP cache. CDP network totals include completed `encodedDataLength`; for an unfinished stream such as SSE, only encoded chunks observed before the snapshot are included and the unfinished-request count remains visible.

The standard contacts scenario records:

- active DOM and JavaScript heap on dashboard and contacts;
- controlled search latency from Enter through the matching response, Angular render, and two animation frames (an end-to-end latency, not field INP);
- approximate `requestAnimationFrame` scrolling FPS, frame intervals, and long tasks across the rendered 50-row page;
- forced-GC JS heap before and after 20 Angular SPA list → details → list cycles.

The last value is explicitly an approximation of retained JavaScript heap using `HeapProfiler.collectGarbage` plus `Runtime.getHeapUsage`; it is not a dominator-tree heap snapshot. A run fails when login, route data, contact rows, or navigation cycles are unavailable, or when the browser emits a console/page error. Change cycle count, viewport, quiet time, output path, and run number only through the documented CLI/environment fields so every artifact retains its method metadata.

The artifact evaluates the documented 150 ms interaction, 1,500-node preferred DOM, 55 FPS scroll, 200 MiB preferred JavaScript heap, and 15% forced-GC growth thresholds without changing them. A failed or preferred-target miss remains in JSON; the runner does not weaken a threshold to make the report green.

Lighthouse is invoked as exact `lighthouse@13.4.1` through pinned pnpm and writes both JSON and HTML under `benchmarks/results/lighthouse/`, plus a small manifest of the reported scores and browser version. Its default target is the public `/login` route, with fresh Lighthouse storage for each profile; it does not silently reuse the authenticated Playwright session. Set `LIGHTHOUSE_PATH` only to another same-origin public route and state that route in the report. Set `LIGHTHOUSE_CHROME_PATH`/`CHROME_PATH` or use the wrapper option when automatic Chrome discovery is unsuitable.

Generated browser artifacts are ignored by Git. Retain the raw files outside the working tree or upload them as CI/release artifacts before cleaning results. For a published central value, run the unchanged scenario at least three times using distinct `BROWSER_BENCHMARK_RUN` and `BROWSER_BENCHMARK_OUTPUT` values and report the median plus every individual result; the default fixed filename intentionally represents only the latest local run.

## Bundle measurement

```bash
pnpm build:web
pnpm bundle:report
```

The scanner compresses emitted JS/CSS with deterministic gzip level 9 and Brotli quality 11, discovers initial assets from the emitted `index.html`, reports every lazy chunk, and rejects external font references. Preserve both `stats.json` and `bundle-report.json`; use stats to map hashed chunks to routes.

## Database profiling

For slow or variable queries, capture `EXPLAIN (ANALYZE, BUFFERS, WAL, SETTINGS, FORMAT JSON)` on synthetic benchmark data using the same parameters and tenant context. Store plans with the report. Check tenant-first index use, rows removed by filters, heap fetches, spills/temp bytes, lock waits, and query count per HTTP operation. Never run `EXPLAIN ANALYZE` against a third party or production tenant without authorization.

## Resource and startup measurement

- Capture `docker compose stats --no-stream app postgres` after at least two minutes idle and during each load run.
- Record container restart/OOM state and database connection counts.
- Measure readiness from process/container start separately from image pull/build time.
- Sample goroutine count and heap only in benchmark mode; profiling endpoints must not be exposed in production.
- Verify the runtime image has no `node` executable with the repository's container verification script.

## Report structure

Every report contains the metadata above, raw-artifact checksums/paths, dataset counts, configured limits, all individual runs, median/p95/p99/throughput/errors, resource/browser/bundle results, threshold table, observed bottleneck, variance, and changes since the previous commit. Use `Not measured` for missing data and explain the exact failed command or unavailable tool.

## Optimization protocol

When a budget fails:

1. Preserve the failing artifact.
2. Form one specific hypothesis from a trace, plan, or profile.
3. Apply optimization A and rerun the unchanged scenario.
4. Apply a distinct optimization B if still over budget (or to validate another major source) and rerun again.
5. Publish before/after values and trade-offs; keep the final miss visible.

Changing the benchmark to conceal a regression invalidates the comparison.
