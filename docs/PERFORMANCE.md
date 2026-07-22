# Performance report

Last updated: 2026-07-21  
Release status: incomplete; no release-final full-stack benchmark has been completed.

## Reporting rules

- Only values produced by a successful command and retained artifact appear as measured.
- `Not measured` is distinct from a failed budget.
- A configured CPU/memory limit is not an observed resource value.
- A historical working-tree artifact is not silently promoted to a current release result.
- k6 results use at least three runs and report the median run/aggregate according to [`BENCHMARK_METHODOLOGY.md`](BENCHMARK_METHODOLOGY.md), never the best run.

## Environment of the recorded bundle artifact

| Field               | Value                                                                                                              |
| ------------------- | ------------------------------------------------------------------------------------------------------------------ |
| Generated at        | `2026-07-21T17:48:10.872Z`                                                                                         |
| Commit SHA          | Not available; repository had no commit and working tree was uncommitted                                           |
| Build method        | Angular production output; Node zlib gzip level 9 and Brotli quality 11 over emitted JS/CSS                        |
| Host OS / CPU / RAM | Not recorded in the artifact                                                                                       |
| Browser             | Not applicable to static compression                                                                               |
| Evidence            | `benchmarks/results/bundle-report.json` (local generated artifact; results directory is ignored except `.gitkeep`) |
| Currency of result  | Historical: frontend source changed after generation; rerun required                                               |

## Recorded frontend bundle result

| Metric                                                     |   Bytes |   KiB |                                Budget | Result for that build         |
| ---------------------------------------------------------- | ------: | ----: | ------------------------------------: | ----------------------------- |
| Initial JS + CSS Brotli                                    |  86,727 |  84.7 | target ≤350 KiB; warn >400; fail >450 | Target met                    |
| Initial JS (`main-G2COUVFD.js`) Brotli                     |  84,249 |  82.3 |                        Included above | Recorded                      |
| Initial CSS (`styles-WSRK6VDY.css`) Brotli                 |   2,478 |   2.4 |                        Included above | Recorded                      |
| Lazy AG Grid chunk (`chunk-DoGjoB7h.js`) Brotli            | 158,063 | 154.4 |   Allowed as lazy grid; document size | Recorded lazy                 |
| Largest ordinary lazy feature (`chunk-I38qxY1g.js`) Brotli |  26,466 |  25.8 |                       ≤200 KiB target | Target met                    |
| External font references                                   |       0 |     — |                                     0 | Target met for scanner inputs |
| Ordinary lazy warnings                                     |       0 |     — |          No ordinary feature >200 KiB | Target met                    |

Chunk hashes are build-specific. The report's content classification is heuristic; route ownership should be correlated with Angular `stats.json` during the release build.

## Current budget matrix

| Metric                          |                    Target | Current evidence                                     | Status                    |
| ------------------------------- | ------------------------: | ---------------------------------------------------- | ------------------------- |
| Release-final initial JS + CSS  |           ≤350 KiB Brotli | Historical value 84.7 KiB; source changed afterwards | Rerun required            |
| Ordinary lazy feature           |           ≤200 KiB Brotli | Historical largest 25.8 KiB                          | Rerun required            |
| Lazy AG Grid/report chunks      |       Lazy and documented | Historical AG Grid 154.4 KiB                         | Rerun required            |
| External font requests          |                         0 | Historical scanner found 0                           | Rerun required in browser |
| Lighthouse desktop performance  |                       ≥95 | Not measured                                         | Not measured              |
| Lighthouse mobile performance   |                       ≥90 | Not measured                                         | Not measured              |
| Lighthouse accessibility        |                       ≥95 | Not measured                                         | Not measured              |
| LCP in fixed mobile emulation   |                    ≤2.0 s | Not measured                                         | Not measured              |
| CLS                             |                     ≤0.05 | Not measured                                         | Not measured              |
| INP/key interaction latency     |                   ≤150 ms | Not measured                                         | Not measured              |
| Active DOM                      |   Preferably ≤1,500 nodes | Not measured                                         | Not measured              |
| Large-grid scrolling            | ≥55 FPS on benchmark host | Not measured                                         | Not measured              |
| Browser heap                    |        Preferably ≤200 MB | Not measured                                         | Not measured              |
| Retained heap growth, 20 cycles |                      ≤15% | Not measured                                         | Not measured              |
| App idle RSS                    |                    ≤96 MB | Not measured                                         | Not measured              |
| Readiness from process start    |                      ≤1 s | Not measured                                         | Not measured              |
| App + PostgreSQL memory         |        Strive for ≤512 MB | Compose limits total 512 MB; observed use absent     | Not measured              |
| Baseline error rate             |                     <0.5% | Not measured                                         | Not measured              |
| Baseline read p95               |                   <150 ms | Not measured                                         | Not measured              |
| Baseline write p95              |                   <250 ms | Not measured                                         | Not measured              |
| Baseline search p95             |                   <250 ms | Not measured                                         | Not measured              |
| Baseline p99 / throughput       |             Always report | Not measured                                         | Not measured              |
| Stretch 100-VU degradation      |         Report bottleneck | Not measured                                         | Not measured              |

## Optimization record

The dated bundle artifact did not cross a frontend budget, so the mandated two-iteration remediation rule was not triggered for that build. It must not be used to avoid optimizing any future regression.

Implementation mechanisms currently intended to control cost:

1. Zoneless OnPush Angular, route-lazy features, scoped signals, tracked loops, cancellable requests.
2. Lazy selective AG Grid Community registration and custom SVG charts instead of another chart/UI kit.
3. Server cursor pagination, bounded Kanban stages, streaming CSV/files, and no full CRM dataset in IndexedDB.
4. Precompressed fingerprinted assets served by the Go binary with immutable cache semantics.
5. Bounded DB pool/worker concurrency, tenant-first indexes, generated SQL, transactional outbox, and leased jobs.

These are design choices, not substitutes for measurement.

## Missing final evidence

The following commands/scenarios still need to complete on a healthy Docker/browser environment:

```bash
pnpm build
pnpm test:e2e
pnpm benchmark
```

Additionally record Lighthouse desktop/mobile runs, a DevTools/CDP heap-retention scenario, large-grid FPS, readiness timing, and `docker compose stats --no-stream app postgres`. Store raw artifacts under `benchmarks/results/` in CI artifacts or an explicitly committed release evidence directory.

Earlier in the build session Docker Desktop's WSL engine returned HTTP 500 and reported an RCU stall. A local PostgreSQL server enabled database integration work during that interruption. Docker later recovered, but production-image, Compose resource, and k6-container values remain unmeasured until their final commands produce retained artifacts.

## Release report template

Copy this table for each retained benchmark report:

| Field                                   | Value |
| --------------------------------------- | ----- |
| Date and UTC time                       |       |
| Commit SHA; clean/dirty                 |       |
| OS/version                              |       |
| CPU/model/governor or power plan        |       |
| RAM                                     |       |
| Docker/Compose version                  |       |
| Browser/version                         |       |
| Dataset profile and verified row counts |       |
| App/PostgreSQL limits                   |       |
| Cold or warm                            |       |
| Warm-up / measured duration             |       |
| Number of runs                          |       |
| Median read/write/search                |       |
| p95/p99 by operation                    |       |
| Throughput/error rate                   |       |
| App/PostgreSQL RSS and CPU              |       |
| Transferred bytes/request count         |       |
| Frontend compressed bundles             |       |
| Lighthouse/LCP/CLS/interaction          |       |
| Browser heap/DOM/FPS/retained growth    |       |
| Raw artifact paths                      |       |
| Observed bottleneck and variance        |       |

## Interpretation

There is not yet enough evidence to claim the server, browser, or two-container profile meets its runtime budgets. The only measured performance evidence is the dated bundle artifact above. No competitor result has been measured.
