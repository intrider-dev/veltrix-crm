# Performance report

Last updated: 2026-07-22
Evidence status: measured from the dirty working tree at commit base `3326e072a37fdfd1f58ccaf5ceb1d98de21f63dc`; results are not presented as a tagged release.

## Reporting rules

- Values below come from retained local artifacts or successful commands. `Not measured` is used when no evidence exists.
- k6 figures are the median of three independent clean-dataset runs, never the best run.
- Browser memory and scrolling numbers are controlled approximations with the method recorded in each artifact.
- Configured container limits are not reported as observed resource use.

## Measurement environment

| Field | Value |
| --- | --- |
| Date | 2026-07-22 |
| Host | Windows `10.0.26200`, AMD Ryzen 7 9800X3D, 16 logical CPUs, 63.6 GiB usable RAM |
| Docker | Engine 29.5.3; Compose 5.1.4 |
| Browser | Headless Chromium 149.0.7827.55 / Chrome 149 for Lighthouse |
| Database | PostgreSQL 18.4, deterministic synthetic `benchmark` profile |
| Baseline limits | app: 0.5 CPU / 128 MiB; PostgreSQL: 0.5 CPU / 384 MiB |
| Load | 50 VUs; 1 minute warm-up plus 5 measured minutes; 3 clean runs |

Raw local artifacts are written under `benchmarks/results/` by the documented scripts and are ignored by Git. Portfolio screenshots copied from a successful real-browser run are retained under `docs/screenshots/`.

## Frontend bundle

Production output was compressed with Node zlib gzip level 9 and Brotli quality 11. Evidence: `benchmarks/results/bundle-report.json`, generated `2026-07-22T15:38:30.879Z`.

| Metric | Measured | Target | Status |
| --- | ---: | ---: | --- |
| Initial JS + CSS, Brotli | 91,782 B / 89.6 KiB | ≤350 KiB | Pass |
| Largest ordinary lazy app chunk, Brotli | 21,065 B / 20.6 KiB | ≤200 KiB | Pass |
| Lazy AG Grid Community chunk, Brotli | 170,685 B / 166.7 KiB | lazy and documented | Pass |
| Optional lazy LiveKit client, Brotli | 116,990 B / 114.2 KiB | optional and lazy | Pass |
| External font references | 0 | 0 | Pass |
| Inline event handlers | 0 | 0 | Pass |

AG Grid and LiveKit are not part of the initial route. The report also verifies that no ordinary lazy chunk exceeds its budget.

## Lighthouse and browser scenario

Lighthouse 13.4.1 measured the production-like `/login` route. The official simulated mobile LCP is used for the budget decision; the faster observed trace value is not substituted.

| Metric | Desktop | Mobile | Target | Status |
| --- | ---: | ---: | ---: | --- |
| Performance | 100 | 94 | ≥95 / ≥90 | Pass |
| Accessibility | 100 | 100 | ≥95 | Pass |
| LCP | 661.52 ms | 2,771.49 ms | mobile ≤2,000 ms | **Miss** |
| CLS | 0 | 0 | ≤0.05 | Pass |
| Total blocking time | 0 ms | 29 ms | recorded | Recorded |

Three browser-performance runs used a fresh authenticated context, warm HTTP cache, blocked service workers for deterministic network accounting, ten warm-up cycles, then twenty list → detail → list cycles. Medians:

| Metric | Median | Target | Status |
| --- | ---: | ---: | --- |
| Controlled contact-search interaction | 46.5 ms | ≤150 ms | Pass |
| Active DOM | 710 nodes | preferably ≤1,500 | Pass |
| Virtualized grid scrolling | 60 FPS approximate | ≥55 FPS | Pass |
| Forced-GC JS heap after scenario | 13.26 MiB | preferably ≤200 MiB | Pass |
| Retained heap growth after 20 cycles | 8.15% | ≤15% | Pass |
| Dashboard warm navigation | 570.53 ms; 52,863 B transferred | recorded | Recorded |
| Contacts warm navigation | 628.67 ms; 208,688 B transferred | recorded | Recorded |
| Browser errors across measured runs | 0 | 0 | Pass |

The FPS value is requestAnimationFrame sampling in headless Chromium, not compositor telemetry. Heap is forced-GC `Runtime.getHeapUsage`, not a dominator-tree snapshot.

## k6 baseline

`pnpm benchmark` completed three isolated runs and generated `benchmarks/results/k6-baseline-summary.json`. The wrapper exited with status 1 because the read and write thresholds missed; the complete summaries and resource samples were still retained. No run had request errors or interrupted iterations.

| Metric | Run 1 | Run 2 | Run 3 | Reported median | Target | Status |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Throughput, operations/s | 224.52 | 222.44 | 223.35 | 223.35 | record | Recorded |
| Error rate | 0% | 0% | 0% | 0% | <0.5% | Pass |
| Read p95 | 188.54 ms | 191.55 ms | 189.94 ms | 189.94 ms | <150 ms | **Miss** |
| Read p99 | 200.40 ms | 203.19 ms | 203.39 ms | 203.19 ms | record | Recorded |
| Write p95 | 284.68 ms | 290.20 ms | 290.14 ms | 290.14 ms | <250 ms | **Miss** |
| Write p99 | 303.84 ms | 394.06 ms | 396.42 ms | 394.06 ms | record | Recorded |
| Search p95 | 159.95 ms | 101.22 ms | 176.99 ms | 159.95 ms | <250 ms | Pass |
| Search p99 | 192.22 ms | 192.02 ms | 191.48 ms | 192.02 ms | record | Recorded |
| Overall p95 / p99 | 191.97 / 206.57 ms | 193.79 / 216.36 ms | 192.89 / 213.61 ms | 192.89 / 213.61 ms | record | Recorded |
| Transferred bytes | 714.7 MB | 702.8 MB | 708.2 MB | 708.2 MB | record | Recorded |

One isolated maximum duration reached approximately 64 seconds in each long Docker Desktop run. The same session produced an impossible BuildKit 63.7-second internal step inside a 25.2-second host-wall build, so a Docker Desktop/VM clock discontinuity is suspected, but not proven. The samples remain in the raw artifacts, no percentile was edited, and they did not change p99 or error rate. The 100-VU stretch scenario is **Not measured**.

## Runtime resources

Median peak values across the three baseline runs:

| Resource | App | PostgreSQL | Status |
| --- | ---: | ---: | --- |
| Memory | 73.65 MiB | 305.20 MiB | combined 378.85 MiB; pass the ≤512 MiB aspiration |
| CPU | 38.78% | 53.58% | recorded under 0.5-CPU limits |
| PIDs | 10 | 28 | bounded |

A separate post-E2E idle snapshot of the final image recorded app 72.65 MiB and PostgreSQL 151.1 MiB; app passed the ≤96 MiB idle target. Readiness log delta was approximately 221 ms, passing the ≤1 second target. These two values are single observations, not three-run medians.

The exported `scratch` runtime filesystem contained the Go application, CA certificates, configuration/directories, and no Node.js/npm/pnpm artifact. Runtime user is `65532:65532`.

## Optimization iterations

1. Earlier query work replaced a tenant search plan that scanned roughly 300,000 RLS-filtered documents, changed contact/company lookup to indexed lateral probes, reduced authorization round trips, and throttled session `last_seen_at` writes. AG Grid rendering and route lifecycle were also bounded. The retained browser runs now show 8.15%, 5.76%, and 8.16% heap growth.
2. A later security-complete regression run exposed a dashboard statement whose stage-authorized fallback branch was planned/JIT-compiled even when a stored summary existed. The pre-fix three-run median was only 40.85 operations/s with read/write/search p95 of 1,590/1,992/1,982 ms. Iteration one split the stored-summary query from the stage-filtered fallback and invokes the fallback only when stage ACL rules exist. A short 50-VU diagnostic then measured 224.94 operations/s and read/write/search p95 of 99.29/187.13/93.13 ms.
3. Iteration two added migration `000035`: search authorization facts are computed once per request, stage checks are bypassed only for the already-proven system-admin path, and candidate rows are bounded proportionally to the requested result limit. A second short diagnostic measured 242.03 operations/s and read/write/search p95 of 97.93/106.51/93.26 ms. The final full-duration, three-clean-run median is the table above; read and write p95 still miss, so neither target was weakened.

## Budget summary

Passed: initial/lazy bundles, external fonts, Lighthouse desktop/mobile performance, accessibility, CLS, interaction latency, DOM, grid FPS, browser heap/retention, error rate, search p95, app idle RSS, readiness, and combined memory aspiration.

Missed: simulated mobile LCP (2.77 s vs 2.0 s), median baseline read p95 (189.94 ms vs 150 ms), and median write p95 (290.14 ms vs 250 ms).

Not measured: 100-VU stretch, competitor products, field INP, and production internet/TLS performance. See [`BENCHMARK_METHODOLOGY.md`](BENCHMARK_METHODOLOGY.md) for reproduction and [`COMPETITOR_BENCHMARK_PROTOCOL.md`](COMPETITOR_BENCHMARK_PROTOCOL.md) for a manual, ToS-respecting comparison protocol.
