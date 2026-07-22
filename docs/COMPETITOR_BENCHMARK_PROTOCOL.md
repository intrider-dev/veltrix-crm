# Manual competitor benchmark protocol

This protocol lets a repository owner compare the project with another CRM under controlled conditions. It contains no competitor results. Every value starts as `Not measured`.

## Legal and ethical boundary

- Read and follow the compared product's Terms of Service, acceptable-use policy, automation policy, and privacy terms.
- Use only accounts, workspaces, and synthetic data you own or are explicitly authorized to test.
- Do not automate, scrape, load-test, bypass controls, or reverse engineer a third-party product without written permission.
- Do not capture another person's data, credentials, proprietary source, or private tenant identifiers.
- Prefer manual browser scenarios and DevTools recordings. Stop if the service shows distress or the provider objects.
- A self-hosted project's license does not automatically authorize testing someone else's hosted instance.

## Fair-comparison controls

Use the same:

- physical computer, OS session, power plan, display scaling, and background-load policy;
- browser/version, clean profile, extension state, viewport, device emulation, locale, and timezone;
- network connection or recorded shaping profile;
- approximate number of contacts, companies, deals, activities, stages, custom fields, and users;
- scenario wording, think time, cache definition, measurement tool, and repetition count;
- date window where possible; record each product/version/build visible to the user.

Exact dataset equivalence may be impossible because products model entities differently. Document every mismatch and do not normalize away a capability that changes the workload.

## Dataset

Create a small, synthetic comparison dataset acceptable under each product's limits. Record exact or approximate counts and how it was loaded. Do not upload the project's 100,000-record benchmark dataset to a third-party service unless its terms and your account plan explicitly allow it.

Recommended minimum for manual interaction:

- 1,000 synthetic contacts;
- 250 synthetic companies;
- 500 synthetic deals across the same number of stages;
- 5,000 synthetic activities if supported;
- equivalent visible columns, filters, and custom fields.

## Repetitions

Perform at least five manual timings per product for each short scenario after one unreported practice run. Report the median, p95 where the sample count makes it meaningful, and every failed run. Alternate product order between rounds to reduce host/network drift. Cold and warm runs are separate populations.

## Scenarios

1. **Cold application load:** fresh browser profile/context and empty cache; login state documented.
2. **Warm application load:** same authenticated user and primed cache.
3. **Open contacts:** navigate from the comparable landing route until usable rows render.
4. **Search contact:** enter the same uncommon synthetic name and wait for a selectable result.
5. **Open details:** select that contact and wait for core fields and timeline.
6. **Create contact:** submit the same fields and wait for durable success/details.
7. **Create deal:** same pipeline, amount, currency, owner, and expected date.
8. **Move deal:** same adjacent stage move and wait for durable confirmation.
9. **Filter list:** apply equivalent owner/status/tag filters and wait for rows.
10. **Dashboard:** load an equivalent period and wait for all primary cards/charts.
11. **Ten-minute memory:** repeat a fixed list/details/pipeline/dashboard loop.
12. **Transfer:** record request count and bytes for cold/warm loads and key flows.
13. **Idle server resources:** only for self-hosted deployments you control.
14. **CPU during load:** only with explicit authorization and equivalent resource limits.

Define “usable” before timing: for example, the heading and first stable data row are visible, no loading overlay blocks interaction, and no required request remains pending.

## Evidence capture

For each product retain where permitted:

- dated DevTools Performance and Network screenshots;
- HAR with sensitive cookies, tokens, query text, and personal data removed;
- trace files only if the product terms allow them;
- browser Task Manager/Memory or DevTools heap screenshots;
- self-hosted container/host stats and exact limits;
- scenario worksheet, raw timings, failures, cache state, and product version.

Sanitize artifacts before publication. A HAR can contain authentication and tenant secrets even when the visible page uses synthetic data.

## Results worksheet

| Scenario                        | This project | Compared product              | Method / evidence | Notes                       |
| ------------------------------- | ------------ | ----------------------------- | ----------------- | --------------------------- |
| Cold application load           | Not measured | Not measured                  |                   |                             |
| Warm application load           | Not measured | Not measured                  |                   |                             |
| Open contacts                   | Not measured | Not measured                  |                   |                             |
| Search contact                  | Not measured | Not measured                  |                   |                             |
| Open details                    | Not measured | Not measured                  |                   |                             |
| Create contact                  | Not measured | Not measured                  |                   |                             |
| Create deal                     | Not measured | Not measured                  |                   |                             |
| Move deal between stages        | Not measured | Not measured                  |                   |                             |
| Filter list                     | Not measured | Not measured                  |                   |                             |
| Dashboard load                  | Not measured | Not measured                  |                   |                             |
| Browser memory after 10 minutes | Not measured | Not measured                  |                   |                             |
| Transferred bytes               | Not measured | Not measured                  |                   |                             |
| Network request count           | Not measured | Not measured                  |                   |                             |
| Idle server memory              | Not measured | Not measured / inaccessible   |                   | Self-hosted only            |
| CPU during authorized load      | Not measured | Not measured / not authorized |                   | Self-hosted/permission only |

## Interpretation rules

- Do not turn a single scenario into a blanket “faster” claim.
- Do not compare a local deployment to a remote SaaS without prominently explaining network/topology differences.
- Do not compare different subscription tiers, cache states, datasets, or feature sets without labeling the mismatch.
- Report correctness, failed requests, missing records, and UI readiness together with time.
- Treat inaccessible server metrics as `Not measured`, never as zero.
- Include the measurement date; third-party products change without notice.

Publication should state that results describe the recorded versions, account configuration, host, dataset, and date only. Re-run before repeating a claim in a later release.
