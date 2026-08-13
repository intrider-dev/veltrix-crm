# Portfolio screenshot evidence

This directory intentionally contains no fabricated product images. Screenshots are accepted only when Playwright captures the real production-like application after the matching browser smoke passes with no console errors.

## Capture

```bash
pnpm test:e2e -- --grep "captures real application portfolio views"
```

Playwright runs the configured 1440×900 desktop, 1024×768 tablet, and 390×844 mobile projects. Raw output is written under `benchmarks/results/playwright/`; review it before copying approved images here.

## Required publication set

Use stable lowercase filenames. Do not place user names, host paths, credentials, cookies, API keys, reset links, real customer data, or debug overlays in an image.

| View                              | Preferred file                  | Status       |
| --------------------------------- | ------------------------------- | ------------ |
| Dashboard, desktop                | `dashboard-1440x900.png`        | Not captured |
| Contacts grid, desktop            | `contacts-grid-1440x900.png`    | Not captured |
| Deal pipeline, desktop            | `deal-pipeline-1440x900.png`    | Not captured |
| Contact/company timeline, desktop | `details-timeline-1440x900.png` | Not captured |
| Reports, desktop                  | `reports-1440x900.png`          | Not captured |
| Dark theme, desktop               | `dashboard-dark-1440x900.png`   | Not captured |
| Representative tablet view        | `dashboard-1024x768.png`        | Not captured |
| Representative mobile view        | `contacts-390x844.png`          | Not captured |

## Acceptance checklist

- Image comes from the current commit and documented synthetic seed.
- Route data and loading have stabilized; no skeleton is accidentally captured unless it is the subject.
- Browser console and failed-request collector are clean.
- Focus, hover, transient toast, and cursor are placed intentionally.
- Text is legible, locale is declared, and no translation key/fallback leaks into UI.
- Light/dark theme and viewport match the filename.
- The image is not edited to add controls, metrics, records, or states that were not present. Cropping and lossless metadata removal are allowed.
- README status changes from `Not captured` only after the file is committed.

Screenshots are evidence of visual state, not performance or accessibility scores. Keep the matching Playwright trace/report as a CI or release artifact.
