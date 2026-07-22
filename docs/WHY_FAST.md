# Why the system is designed to be fast

This document explains mechanisms and falsifiable expectations. It does not claim that the product is faster than another CRM. Actual results and missed budgets belong in [`PERFORMANCE.md`](PERFORMANCE.md).

## Performance model

User-perceived latency is treated as the sum of several bounded costs:

```text
interaction latency
  = input scheduling
  + rendered DOM work
  + request/transfer
  + authorization and SQL
  + response parsing/state update
```

Optimizing one term while allowing another to grow without bounds does not produce a fast CRM. The architecture therefore controls browser payload/DOM/heap, network page sizes, database query shape, background concurrency, and deployment overhead together.

## Frontend decisions

### Small initial shell

Every major feature is route-lazy. Contacts load AG Grid Community only on list routes; reports and complex editors do not inflate login/dashboard startup. Material/CDK imports are selective, charts are small inline SVG components, and the application requests neither external fonts nor an icon font.

An emitted-asset scanner calculates gzip and Brotli sizes from the production output instead of relying only on Angular's raw transfer estimate. It enforces the 450 KiB hard initial failure and reports ordinary lazy chunks over 200 KiB.

### Explicit change propagation

Angular runs zoneless with OnPush standalone components. Signals and computed state invalidate only consumers that use them. Feature-scoped stores avoid copying all CRM records into a global client store. Effects are reserved for actual side effects, while stale HTTP work is cancellable.

### Bounded rendering and state

- Large lists use server cursors/infinite data sources; the browser never loads 100,000 contacts.
- AG Grid renders a small viewport and reuses rows.
- Kanban requests a bounded page per stage instead of the entire pipeline.
- Lists use stable tracking keys and details are lazy routes/drawers.
- IndexedDB stores drafts and small recent metadata, not a database replica.
- Feature caches, SSE replay, and live-client queues have explicit bounds.

### Perceived responsiveness without hiding failures

Optimistic mutations are used only where rollback is well defined—for example, moving a versioned deal between stages. Skeletons preserve layout; important errors remain visible; repeated searches debounce for roughly 150 ms and cancel obsolete requests. Reduced-motion users do not pay for decorative transitions.

## Network decisions

- Same-origin deployment avoids a second origin, preflight traffic, and browser token storage.
- Fingerprinted assets receive immutable caching and have prebuilt Brotli/gzip variants.
- REST collections use opaque cursors and small default limits.
- Conditional requests use ETag/`If-None-Match`; edits use ETag/`If-Match` rather than retransmitting or silently overwriting state.
- CSV exports and attachment transfer stream rather than buffering the whole object.
- SSE replaces repeated notification polling and supports reconnect/replay.
- Problem responses carry compact stable codes and structured parameters; translation catalogs stay in lazy application chunks.

Transferred bytes and request counts are benchmark outputs, not assumed wins.

## Backend decisions

### A small process surface

The Go standard library supplies HTTP, structured logging, streaming, and shutdown behavior. `chi` and `pgx` are small focused dependencies; generated SQL avoids ORM reflection and hidden N+1 behavior. The runtime does not contain Node.js.

### Bounded resources

- Database connections and worker concurrency are configurable and capped.
- Request bodies, CSV batches, attachment sizes, job batches, retries, SSE buffers, and caches are bounded.
- Worker handlers have deadlines and leases; exponential retry prevents tight failure loops.
- Graceful shutdown stops accepting work, cancels handlers, and closes pools.
- `pprof` is limited to development/benchmark mode; production metrics can remain disabled.

### Query shape

Tenant-owned indexes begin with `workspace_id`. Cursor pagination follows stable indexed columns. Search uses tenant-filtered `tsvector`/trigram indexes. Batch loading and reporting queries are designed to avoid per-row round trips. Money and time values require no locale parsing in SQL.

Database plans still need evidence on both small and benchmark datasets. An index being present does not prove that PostgreSQL chooses it.

## PostgreSQL as infrastructure consolidation

Domain rows, outbox, jobs, search documents, retry state, automation logs, and SSE replay live in one durable system. A domain change and its outbox intent share a transaction; no distributed acknowledgement protocol is needed. This removes required Redis/search/broker processes and their connections, health checks, buffers, and failure modes.

Consolidation moves more work into PostgreSQL, so it is a trade-off rather than a universal optimization. The 50-VU baseline and 100-VU stretch scenarios must expose queue, pool, CPU, WAL, and query contention.

## Build and runtime decisions

The Angular compiler and Node.js exist only in a build stage. The web output is precompressed, copied into a statically compiled Go binary, and delivered from a non-root `scratch` image. Base Compose has two services with explicit CPU, memory, PID, pool, and worker caps.

Configured limits describe the test envelope. Actual RSS, startup, CPU, and OOM behavior remain measured outputs.

## Performance budgets

| Layer                                      | Target                                       |
| ------------------------------------------ | -------------------------------------------- |
| Initial JS + CSS                           | ≤350 KiB Brotli target; warn >400; fail >450 |
| Ordinary lazy feature                      | ≤200 KiB Brotli                              |
| Lighthouse desktop/mobile                  | ≥95 / ≥90                                    |
| Lighthouse accessibility                   | ≥95                                          |
| Mobile-emulation LCP / CLS                 | ≤2.0 s / ≤0.05                               |
| Key interaction latency                    | ≤150 ms                                      |
| Active DOM                                 | preferably ≤1,500 nodes                      |
| Large-list scroll                          | ≥55 FPS on the recorded machine              |
| Browser heap                               | preferably ≤200 MB                           |
| Retained growth over 20 list/detail cycles | ≤15%                                         |
| App idle RSS / readiness                   | ≤96 MB / ≤1 s                                |
| Baseline error rate                        | <0.5%                                        |
| Baseline read/write/search p95             | <150 / <250 / <250 ms                        |

The historical bundle artifact met its applicable build targets. All browser, load, and runtime values remain `Not measured` until their documented scenarios complete.

## How this can be disproved

The design has failed a target if the fixed scenario misses it across repeated runs. The response is to preserve raw artifacts, perform at least two materially different optimization iterations, rerun the identical scenario, and document the final result even if it remains red. Acceptable examples include reducing a route's imports, correcting an SQL plan, changing a bounded batch size, or eliminating retained subscriptions. Lowering VUs, shrinking the dataset, discarding errors, or publishing the best run is not optimization.
