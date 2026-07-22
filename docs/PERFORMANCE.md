# Performance report

Last updated: 2026-07-22
Evidence status: serving-performance application code is fixed by commit `3b26934ba388b7289c400733a46001d6acae6108`; the clean three-run k6 baseline was executed from documentation-only descendant `feaffdda59244d950578c452beb2a835534e2a68`. Browser-performance artifacts report the same serving code. Bundle evidence was regenerated from the final working tree after deployment/i18n hardening that did not alter measured request/query paths. Results are not presented as a tagged release.

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

Production output was compressed with Node zlib gzip level 9 and Brotli quality 11. Evidence: `benchmarks/results/bundle-report.json`, generated `2026-07-22T17:11:51.420Z`.

| Metric | Measured | Target | Status |
| --- | ---: | ---: | --- |
| Initial JS + CSS, Brotli | 91,712 B / 89.6 KiB | ≤350 KiB | Pass |
| Largest ordinary lazy app chunk, Brotli | 21,041 B / 20.5 KiB | ≤200 KiB | Pass |
| Lazy AG Grid Community chunk, Brotli | 170,706 B / 166.7 KiB | lazy and documented | Pass |
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
| LCP | 662.22 ms | 2,762.41 ms | mobile ≤2,000 ms | **Miss** |
| CLS | 0 | 0 | ≤0.05 | Pass |
| Total blocking time | 0 ms | 12.5 ms | recorded | Recorded |

Three browser-performance runs used a fresh authenticated context, warm HTTP cache, blocked service workers for deterministic network accounting, ten warm-up cycles, then twenty list → detail → list cycles. Medians:

| Metric | Median | Target | Status |
| --- | ---: | ---: | --- |
| Controlled contact-search interaction | 49.1 ms | ≤150 ms | Pass |
| Active DOM | 710 nodes | preferably ≤1,500 | Pass |
| Virtualized grid scrolling | 60 FPS approximate | ≥55 FPS | Pass |
| Forced-GC JS heap after scenario | 13.30 MiB | preferably ≤200 MiB | Pass |
| Retained heap growth after 20 cycles | 8.3% | ≤15% | Pass |
| Dashboard warm navigation | 587.14 ms; 54,704 B transferred | recorded | Recorded |
| Contacts warm navigation | 619.70 ms; 210,344 B transferred | recorded | Recorded |
| Browser errors across measured runs | 0 | 0 | Pass |

The FPS value is requestAnimationFrame sampling in headless Chromium, not compositor telemetry. Heap is forced-GC `Runtime.getHeapUsage`, not a dominator-tree snapshot.

## k6 baseline

`pnpm benchmark` completed three isolated runs and generated `benchmarks/results/k6-baseline-summary.json`. The wrapper exited with status 1 because the read and write thresholds missed; the complete summaries and resource samples were still retained. No run had request errors or interrupted iterations.

| Metric | Run 1 | Run 2 | Run 3 | Reported median | Target | Status |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Throughput, operations/s | 222.61 | 214.24 | 227.69 | 222.61 | record | Recorded |
| Error rate | 0% | 0% | 0% | 0% | <0.5% | Pass |
| Read p95 | 189.09 ms | 194.45 ms | 186.31 ms | 189.09 ms | <150 ms | **Miss** |
| Read p99 | 201.09 ms | 201.23 ms | 199.86 ms | 201.09 ms | record | Recorded |
| Write p95 | 283.19 ms | 202.00 ms | 284.10 ms | 283.19 ms | <250 ms | **Miss** |
| Write p99 | 312.94 ms | 295.61 ms | 339.00 ms | 312.94 ms | record | Recorded |
| Search p95 | 176.65 ms | 186.94 ms | 100.67 ms | 176.65 ms | <250 ms | Pass |
| Search p99 | 192.48 ms | 195.15 ms | 190.84 ms | 192.48 ms | record | Recorded |
| Overall p95 / p99 | 191.59 / 207.09 ms | 194.31 / 201.46 ms | 190.89 / 206.95 ms | 191.59 / 206.95 ms | record | Recorded |
| Transferred bytes | 704.6 MB | 683.8 MB | 724.5 MB | 704.6 MB | record | Recorded |

One isolated maximum duration reached approximately 64 seconds in each long Docker Desktop run. The same session produced an impossible BuildKit 63.7-second internal step inside a 25.2-second host-wall build, so a Docker Desktop/VM clock discontinuity is suspected, but not proven. The samples remain in the raw artifacts, no percentile was edited, and they did not change p99 or error rate. The 100-VU stretch scenario is **Not measured**.

## Runtime resources

Median peak values across the three baseline runs:

| Resource | App | PostgreSQL | Status |
| --- | ---: | ---: | --- |
| Memory | 72.14 MiB | 306.10 MiB | combined 378.24 MiB; pass the ≤512 MiB aspiration |
| CPU | 38.09% | 53.78% | recorded under 0.5-CPU limits |
| PIDs | 9 | 28 | bounded |

A separate post-E2E idle snapshot after 36 minutes of final-image uptime recorded app 12.61 MiB and PostgreSQL 59.74 MiB at 0.19% CPU each; app passed the ≤96 MiB idle target. Readiness log delta was approximately 221 ms, passing the ≤1 second target. These values are single observations, not three-run medians.

The exported `scratch` runtime filesystem contained the Go application, CA certificates, configuration/directories, and no Node.js/npm/pnpm artifact. Runtime user is `65532:65532`.

## Optimization iterations

1. Earlier query work replaced a tenant search plan that scanned roughly 300,000 RLS-filtered documents, changed contact/company lookup to indexed lateral probes, reduced authorization round trips, and throttled session `last_seen_at` writes. AG Grid rendering and route lifecycle were also bounded. The final retained browser runs show 4.0%, 8.3%, and 8.6% heap growth.
2. A later security-complete regression run exposed a dashboard statement whose stage-authorized fallback branch was planned/JIT-compiled even when a stored summary existed. The pre-fix three-run median was only 40.85 operations/s with read/write/search p95 of 1,590/1,992/1,982 ms. Iteration one split the stored-summary query from the stage-filtered fallback and invokes the fallback only when stage ACL rules exist. A short 50-VU diagnostic then measured 224.94 operations/s and read/write/search p95 of 99.29/187.13/93.13 ms.
3. Iteration two added migration `000035`: search authorization facts are computed once per request, stage checks are bypassed only for the already-proven system-admin path, and candidate rows are bounded proportionally to the requested result limit. A second short diagnostic measured 242.03 operations/s and read/write/search p95 of 97.93/106.51/93.26 ms. The final full-duration, three-clean-run median is the table above; read and write p95 still miss, so neither target was weakened.

## Budget summary

Passed: initial/lazy bundles, external fonts, Lighthouse desktop/mobile performance, accessibility, CLS, interaction latency, DOM, grid FPS, browser heap/retention, error rate, search p95, app idle RSS, readiness, and combined memory aspiration.

Missed: simulated mobile LCP (2.76 s vs 2.0 s), median baseline read p95 (189.09 ms vs 150 ms), and median write p95 (283.19 ms vs 250 ms).

Not measured: 100-VU stretch, competitor products, field INP, and production internet/TLS performance. See [`BENCHMARK_METHODOLOGY.md`](BENCHMARK_METHODOLOGY.md) for reproduction and [`COMPETITOR_BENCHMARK_PROTOCOL.md`](COMPETITOR_BENCHMARK_PROTOCOL.md) for a manual, ToS-respecting comparison protocol.
