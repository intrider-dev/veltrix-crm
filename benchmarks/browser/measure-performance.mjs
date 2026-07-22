import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import os from "node:os";
import path from "node:path";
import { performance as nodePerformance } from "node:perf_hooks";
import { fileURLToPath, pathToFileURL } from "node:url";
import { mkdirSync, writeFileSync } from "node:fs";

const require = createRequire(import.meta.url);
const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const defaultOutput = path.join(
  repositoryRoot,
  "benchmarks",
  "results",
  "browser-performance.json",
);

const helpText = `VeltrixCRM browser performance measurement

Usage:
  node benchmarks/browser/measure-performance.mjs [options]

Options:
  --base-url <url>       Production-like application URL
  --output <path>        JSON artifact (default: benchmarks/results/browser-performance.json)
  --cycles <count>       SPA list -> details -> list cycles (default: 20)
  --viewport <WxH>       Chromium viewport (default: 1440x900)
  --settle-ms <ms>       Quiet time after application readiness (default: 500)
  --scroll-ms <ms>       Controlled grid scroll duration (default: 3000)
  --run <number>         Run number stored in metadata (default: 1)
  --headed               Run visible Chromium instead of headless Chromium
  --help                 Print this message without launching a browser

Environment equivalents:
  BROWSER_BENCHMARK_BASE_URL, E2E_BASE_URL, E2E_EMAIL, E2E_PASSWORD,
  BROWSER_BENCHMARK_OUTPUT, BROWSER_BENCHMARK_CYCLES,
  BROWSER_BENCHMARK_VIEWPORT, BROWSER_BENCHMARK_SETTLE_MS,
  BROWSER_BENCHMARK_SCROLL_MS, BROWSER_BENCHMARK_RUN, BROWSER_BENCHMARK_HEADED
`;

export function parseViewport(value) {
  const match = /^(\d{2,5})x(\d{2,5})$/i.exec(value);
  if (!match)
    throw new Error("viewport must use WIDTHxHEIGHT, for example 1440x900");
  const width = Number(match[1]);
  const height = Number(match[2]);
  if (width < 320 || width > 7680 || height < 320 || height > 4320) {
    throw new Error("viewport must be between 320x320 and 7680x4320");
  }
  return { width, height };
}

function boundedInteger(value, label, minimum, maximum) {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < minimum || parsed > maximum) {
    throw new Error(
      `${label} must be an integer between ${minimum} and ${maximum}`,
    );
  }
  return parsed;
}

export function normalizedBaseURL(value) {
  const url = new URL(value);
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new Error("base URL must use http or https");
  }
  if (url.username || url.password)
    throw new Error("base URL must not contain credentials");
  url.hash = "";
  url.search = "";
  return url.href.replace(/\/$/, "");
}

export function percentile(values, ratio) {
  if (values.length === 0) return 0;
  const ordered = [...values].sort((left, right) => left - right);
  const index = Math.min(
    ordered.length - 1,
    Math.ceil(ordered.length * ratio) - 1,
  );
  return ordered[Math.max(0, index)];
}

export function parseArguments(argv, environment = process.env) {
  const options = {
    baseURL:
      environment.BROWSER_BENCHMARK_BASE_URL ??
      environment.E2E_BASE_URL ??
      "http://127.0.0.1:8080",
    output: environment.BROWSER_BENCHMARK_OUTPUT ?? defaultOutput,
    cycles: environment.BROWSER_BENCHMARK_CYCLES ?? "20",
    viewport: environment.BROWSER_BENCHMARK_VIEWPORT ?? "1440x900",
    settleMs: environment.BROWSER_BENCHMARK_SETTLE_MS ?? "500",
    scrollMs: environment.BROWSER_BENCHMARK_SCROLL_MS ?? "3000",
    run: environment.BROWSER_BENCHMARK_RUN ?? "1",
    headed: environment.BROWSER_BENCHMARK_HEADED === "true",
    help: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--") continue;
    if (argument === "--help" || argument === "-h") options.help = true;
    else if (argument === "--headed") options.headed = true;
    else {
      const value = argv[index + 1];
      if (!value || value.startsWith("--"))
        throw new Error(`${argument} requires a value`);
      index += 1;
      switch (argument) {
        case "--base-url":
          options.baseURL = value;
          break;
        case "--output":
          options.output = value;
          break;
        case "--cycles":
          options.cycles = value;
          break;
        case "--viewport":
          options.viewport = value;
          break;
        case "--settle-ms":
          options.settleMs = value;
          break;
        case "--scroll-ms":
          options.scrollMs = value;
          break;
        case "--run":
          options.run = value;
          break;
        default:
          throw new Error(`unknown option: ${argument}`);
      }
    }
  }

  return {
    baseURL: normalizedBaseURL(options.baseURL),
    output: path.resolve(repositoryRoot, options.output),
    cycles: boundedInteger(options.cycles, "cycles", 1, 100),
    viewport: parseViewport(options.viewport),
    settleMs: boundedInteger(options.settleMs, "settle-ms", 0, 30_000),
    scrollMs: boundedInteger(options.scrollMs, "scroll-ms", 500, 60_000),
    run: boundedInteger(options.run, "run", 1, 10_000),
    headed: options.headed,
    help: options.help,
    email: environment.E2E_EMAIL ?? "admin@demo.local",
    password: environment.E2E_PASSWORD ?? "Demo123!",
  };
}

function round(value, digits = 2) {
  const multiplier = 10 ** digits;
  return Math.round(value * multiplier) / multiplier;
}

function readGit(command) {
  try {
    return execFileSync("git", command, {
      cwd: repositoryRoot,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return "unavailable";
  }
}

function sanitizedConsoleMessage(message) {
  return message
    .replace(/([?&](?:token|secret|code|key)=)[^&\s]+/gi, "$1[redacted]")
    .replace(/(authorization:\s*)(?:bearer\s+)?\S+/gi, "$1[redacted]")
    .slice(0, 500);
}

function createNetworkTracker(client) {
  let active = null;
  const requestOwners = new Map();

  client.on("Network.requestWillBeSent", (event) => {
    if (!active) return;
    const request = {
      completedEncodedBytes: null,
      streamedEncodedBytes: 0,
      failed: false,
      fromCache: false,
      fromServiceWorker: false,
      resourceType: event.type ?? "Other",
      status: null,
    };
    active.requests.push(request);
    requestOwners.set(event.requestId, { collector: active, request });
  });

  client.on("Network.responseReceived", (event) => {
    const owner = requestOwners.get(event.requestId);
    if (!owner) return;
    owner.request.fromCache = Boolean(event.response.fromDiskCache);
    owner.request.fromServiceWorker = Boolean(event.response.fromServiceWorker);
    owner.request.status = event.response.status;
  });

  client.on("Network.requestServedFromCache", (event) => {
    const owner = requestOwners.get(event.requestId);
    if (owner) owner.request.fromCache = true;
  });

  client.on("Network.dataReceived", (event) => {
    const owner = requestOwners.get(event.requestId);
    if (owner)
      owner.request.streamedEncodedBytes += event.encodedDataLength ?? 0;
  });

  client.on("Network.loadingFinished", (event) => {
    const owner = requestOwners.get(event.requestId);
    if (!owner) return;
    owner.request.completedEncodedBytes = event.encodedDataLength;
    requestOwners.delete(event.requestId);
  });

  client.on("Network.loadingFailed", (event) => {
    const owner = requestOwners.get(event.requestId);
    if (!owner) return;
    owner.request.failed = true;
    requestOwners.delete(event.requestId);
  });

  return {
    begin(label) {
      if (active)
        throw new Error(`network collector ${active.label} is already active`);
      active = { label, requests: [] };
      return active;
    },
    finish(collector) {
      if (active !== collector)
        throw new Error(`network collector ${collector.label} is not active`);
      active = null;
      const unfinished = collector.requests.filter(
        (request) => request.completedEncodedBytes === null && !request.failed,
      );
      for (const [requestId, owner] of requestOwners) {
        if (owner.collector === collector) requestOwners.delete(requestId);
      }
      const resourceTypes = {};
      for (const request of collector.requests) {
        resourceTypes[request.resourceType] =
          (resourceTypes[request.resourceType] ?? 0) + 1;
      }
      return {
        requestCount: collector.requests.length,
        transferredBytes: Math.round(
          collector.requests.reduce(
            (total, request) =>
              total +
              (request.completedEncodedBytes ?? request.streamedEncodedBytes),
            0,
          ),
        ),
        completedRequestCount: collector.requests.filter(
          (request) => request.completedEncodedBytes !== null,
        ).length,
        unfinishedRequestCount: unfinished.length,
        failedRequestCount: collector.requests.filter(
          (request) => request.failed,
        ).length,
        cacheHitCount: collector.requests.filter((request) => request.fromCache)
          .length,
        serviceWorkerResponseCount: collector.requests.filter(
          (request) => request.fromServiceWorker,
        ).length,
        resourceTypes,
        note: "Transferred bytes use CDP encodedDataLength. For unfinished streams such as SSE, only data chunks observed before the route snapshot are counted.",
      };
    },
  };
}

async function waitForAppReady(page, settleMs) {
  await page.locator("main#main-content").waitFor({ state: "visible" });
  await page.waitForFunction(
    () => document.querySelectorAll(".skeleton").length === 0,
    null,
    {
      timeout: 30_000,
    },
  );
  if (settleMs > 0) await page.waitForTimeout(settleMs);
}

async function ensureEnglishApplicationLocale(page, baseURL, settleMs) {
  let locale = await page.evaluate(() => document.documentElement.lang);
  if (!locale.toLowerCase().startsWith("en")) {
    await page.evaluate(async () => {
      const csrf = document.cookie
        .split(";")
        .map((part) => part.trim())
        .find((part) => part.startsWith("XSRF-TOKEN="))
        ?.slice("XSRF-TOKEN=".length);
      if (!csrf) throw new Error("the readable CSRF cookie is missing");
      const response = await fetch("/api/v1/me", {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": decodeURIComponent(csrf),
        },
        body: JSON.stringify({ preferredLocale: "en" }),
      });
      if (!response.ok) {
        throw new Error(
          `preferred-locale update returned HTTP ${response.status}`,
        );
      }
    });
    await page.goto(`${baseURL}/dashboard`, { waitUntil: "domcontentloaded" });
    await waitForAppReady(page, settleMs);
    locale = await page.evaluate(() => document.documentElement.lang);
  }
  if (!locale.toLowerCase().startsWith("en")) {
    throw new Error(
      `application locale remained ${JSON.stringify(locale)} instead of English`,
    );
  }
  return locale;
}

function isWorkspaceCollectionResponse(response, collection) {
  if (response.request().method() !== "GET") return false;
  const url = new URL(response.url());
  return new RegExp(`/api/v1/workspaces/[^/]+/${collection}$`).test(
    url.pathname,
  );
}

async function heapUsage(client, collectGarbage) {
  if (collectGarbage) {
    await client.send("HeapProfiler.collectGarbage");
    await new Promise((resolve) => setTimeout(resolve, 100));
  }
  const usage = await client.send("Runtime.getHeapUsage");
  return {
    usedBytes: Math.round(usage.usedSize),
    totalBytes: Math.round(usage.totalSize),
    embedderUsedBytes:
      typeof usage.embedderHeapUsedSize === "number"
        ? Math.round(usage.embedderHeapUsedSize)
        : null,
    backingStorageBytes:
      typeof usage.backingStorageSize === "number"
        ? Math.round(usage.backingStorageSize)
        : null,
  };
}

async function pageSnapshot(page, client) {
  const [dom, heap, performanceMetrics] = await Promise.all([
    page.evaluate(() => ({
      activeDomNodes: document.querySelectorAll("*").length,
      documentHeight: document.documentElement.scrollHeight,
      viewportHeight: window.innerHeight,
    })),
    heapUsage(client, false),
    client.send("Performance.getMetrics"),
  ]);
  const metric = (name) =>
    performanceMetrics.metrics.find((candidate) => candidate.name === name)
      ?.value ?? null;
  return {
    ...dom,
    jsHeap: heap,
    cdpNodeCount: metric("Nodes"),
    cdpDocumentCount: metric("Documents"),
  };
}

async function measureRoute({
  page,
  client,
  network,
  baseURL,
  route,
  apiCollection,
  settleMs,
}) {
  const collector = network.begin(route);
  const apiResponse = page.waitForResponse(
    (response) => isWorkspaceCollectionResponse(response, apiCollection),
    { timeout: 30_000 },
  );
  const started = nodePerformance.now();
  await page.goto(`${baseURL}${route}`, { waitUntil: "domcontentloaded" });
  const response = await apiResponse;
  if (!response.ok())
    throw new Error(`${route} data request returned HTTP ${response.status()}`);
  await waitForAppReady(page, settleMs);
  const durationMs = nodePerformance.now() - started;
  const snapshot = await pageSnapshot(page, client);
  return {
    route,
    navigationMode: "authenticated document navigation with warm browser cache",
    durationMs: round(durationMs),
    network: network.finish(collector),
    ...snapshot,
  };
}

async function waitForContactRows(page) {
  const grid = page.getByRole("grid");
  await grid.waitFor({ state: "visible", timeout: 30_000 });
  // AG Grid exposes its column-header row first; index one is the first data row.
  await grid
    .getByRole("row")
    .nth(1)
    .waitFor({ state: "visible", timeout: 30_000 });
}

async function runSearchInteraction(page, settleMs) {
  const input = page.locator('form[role="search"] input[type="search"]');
  await input.waitFor({ state: "visible" });
  await input.fill("Synthetic");
  const responsePromise = page.waitForResponse(
    (response) => {
      if (!isWorkspaceCollectionResponse(response, "contacts")) return false;
      return new URL(response.url()).searchParams.get("q") === "Synthetic";
    },
    { timeout: 30_000 },
  );
  const started = await page.evaluate(() => performance.now());
  await input.press("Enter");
  const response = await responsePromise;
  if (!response.ok())
    throw new Error(`contact search returned HTTP ${response.status()}`);
  // The configured quiet period is useful between route measurements, but it
  // is not application work and must not be charged to interaction latency.
  // Keep the real readiness checks (visible main content, no skeletons) and
  // the two post-render animation frames in the measured interval.
  await waitForAppReady(page, 0);
  const finished = await page.evaluate(
    () =>
      new Promise((resolve) =>
        requestAnimationFrame(() =>
          requestAnimationFrame(() => resolve(performance.now())),
        ),
      ),
  );

  await input.fill("");
  const resetPromise = page.waitForResponse(
    (candidate) => {
      if (!isWorkspaceCollectionResponse(candidate, "contacts")) return false;
      return !new URL(candidate.url()).searchParams.has("q");
    },
    { timeout: 30_000 },
  );
  await input.press("Enter");
  const resetResponse = await resetPromise;
  if (!resetResponse.ok())
    throw new Error(
      `contact search reset returned HTTP ${resetResponse.status()}`,
    );
  await waitForAppReady(page, settleMs);
  await waitForContactRows(page);

  return {
    name: "contacts search submit to second post-response animation frame",
    query: "Synthetic",
    latencyMs: round(finished - started),
    method:
      "Browser performance.now() from Enter submission through the matching API response, Angular render completion, and two requestAnimationFrame callbacks.",
    classification:
      "controlled end-to-end interaction latency; not a field INP value",
  };
}

async function measureScroll(page, durationMs) {
  return page.evaluate(async (configuredDurationMs) => {
    const grid = document.querySelector('[role="grid"]');
    const scroller = document.scrollingElement;
    if (!(grid instanceof HTMLElement) || !scroller)
      throw new Error("contacts grid is unavailable");
    const gridTop = grid.getBoundingClientRect().top + scroller.scrollTop;
    const start = Math.max(
      0,
      Math.min(scroller.scrollHeight - innerHeight, gridTop - 80),
    );
    const end = Math.max(
      start,
      Math.min(
        scroller.scrollHeight - innerHeight,
        gridTop + grid.offsetHeight - innerHeight + 80,
      ),
    );
    if (end - start < 100)
      throw new Error(
        "contacts grid does not expose a measurable scroll range",
      );

    const longTasks = [];
    const observer =
      typeof PerformanceObserver !== "undefined" &&
      PerformanceObserver.supportedEntryTypes.includes("longtask")
        ? new PerformanceObserver((list) => {
            for (const entry of list.getEntries())
              longTasks.push(entry.duration);
          })
        : null;
    observer?.observe({ entryTypes: ["longtask"] });
    scroller.scrollTop = start;
    await new Promise((resolve) =>
      requestAnimationFrame(() => requestAnimationFrame(resolve)),
    );

    const frames = [];
    const started = performance.now();
    await new Promise((resolve) => {
      const frame = (now) => {
        frames.push(now);
        const progress = Math.min(1, (now - started) / configuredDurationMs);
        const oscillation = (1 - Math.cos(progress * Math.PI * 4)) / 2;
        scroller.scrollTop = start + (end - start) * oscillation;
        if (progress < 1) requestAnimationFrame(frame);
        else resolve();
      };
      requestAnimationFrame(frame);
    });
    observer?.disconnect();
    scroller.scrollTop = start;

    const intervals = frames
      .slice(1)
      .map((value, index) => value - frames[index]);
    const elapsed =
      frames.length > 1 ? frames.at(-1) - frames[0] : configuredDurationMs;
    return {
      frameCount: frames.length,
      measuredDurationMs: elapsed,
      approximateFps:
        frames.length > 1 ? ((frames.length - 1) * 1000) / elapsed : 0,
      frameIntervalsMs: intervals,
      longTaskDurationsMs: longTasks,
      scrollDistancePx: end - start,
    };
  }, durationMs);
}

function summarizeScroll(raw) {
  return {
    method:
      "requestAnimationFrame-driven two-pass document scroll across the rendered 50-row contacts grid in headless Chromium; this is an approximate rendering signal, not compositor telemetry",
    measuredDurationMs: round(raw.measuredDurationMs),
    scrollDistancePx: Math.round(raw.scrollDistancePx),
    frameCount: raw.frameCount,
    approximateFps: round(raw.approximateFps),
    medianFrameIntervalMs: round(percentile(raw.frameIntervalsMs, 0.5)),
    p95FrameIntervalMs: round(percentile(raw.frameIntervalsMs, 0.95)),
    framesOver50Ms: raw.frameIntervalsMs.filter((duration) => duration > 50)
      .length,
    longTaskCount: raw.longTaskDurationsMs.length,
    longTaskTotalMs: round(
      raw.longTaskDurationsMs.reduce((total, duration) => total + duration, 0),
    ),
  };
}

async function navigateContactRoundTrip(page, settleMs, label) {
  const dataRow = page.getByRole("grid").getByRole("row").nth(1);
  // The final data cell cannot be the optional selection-checkbox cell and
  // therefore exercises the same row-open behavior in every permission mode.
  const cell = dataRow.getByRole("gridcell").last();
  await cell.click();
  await page.waitForURL(/\/contacts\/[0-9a-f-]+$/i, { timeout: 30_000 });
  await waitForAppReady(page, settleMs);

  const contactsResponse = page.waitForResponse(
    (response) => isWorkspaceCollectionResponse(response, "contacts"),
    { timeout: 30_000 },
  );
  await page.locator("a.back-link").click();
  await page.waitForURL(/\/contacts$/, { timeout: 30_000 });
  const response = await contactsResponse;
  if (!response.ok()) {
    throw new Error(`${label} contacts response was HTTP ${response.status()}`);
  }
  await waitForAppReady(page, settleMs);
  await waitForContactRows(page);
}

async function runNavigationCycles(page, client, cycles, settleMs) {
  await waitForContactRows(page);
  // Load the lazy details route, its locale catalog, and stable Angular caches
  // before establishing the retained-heap baseline. The warm-up is explicit
  // in the artifact and is not counted among the requested measured cycles.
  await navigateContactRoundTrip(page, settleMs, "warm-up cycle");
  const before = await heapUsage(client, true);
  const started = nodePerformance.now();
  let completed = 0;
  for (let cycle = 0; cycle < cycles; cycle += 1) {
    await navigateContactRoundTrip(page, settleMs, `cycle ${cycle + 1}`);
    completed += 1;
  }
  const durationMs = nodePerformance.now() - started;
  const after = await heapUsage(client, true);
  const growthBytes = after.usedBytes - before.usedBytes;
  const growthPercent =
    before.usedBytes === 0 ? null : (growthBytes / before.usedBytes) * 100;
  return {
    method:
      "One unmeasured warm-up round-trip loads lazy route code and stable catalogs, then CDP HeapProfiler.collectGarbage plus Runtime.getHeapUsage is sampled before and after repeated Angular SPA navigations; this approximates retained JS heap and is not a dominator-tree heap snapshot",
    warmupCycles: 1,
    requestedCycles: cycles,
    completedCycles: completed,
    totalDurationMs: round(durationMs),
    beforeForcedGc: before,
    afterForcedGc: after,
    usedHeapGrowthBytes: growthBytes,
    usedHeapGrowthPercent: growthPercent === null ? null : round(growthPercent),
  };
}

function metadata(browserVersion, playwrightVersion, options, startedAt) {
  const cpus = os.cpus();
  const status = readGit(["status", "--porcelain"]);
  return {
    startedAt,
    runNumber: options.run,
    baseURL: options.baseURL,
    routeProfile: ["/dashboard", "/contacts", "/contacts/:id"],
    cacheState:
      "fresh browser context, authenticated once, then warm HTTP cache",
    serviceWorkers: "blocked for deterministic direct-network accounting",
    browserContextLocale: "en-US",
    applicationLocale: null,
    timezone: "UTC",
    viewport: options.viewport,
    headless: !options.headed,
    browser: { name: "chromium", version: browserVersion },
    playwrightVersion,
    nodeVersion: process.version,
    host: {
      platform: os.platform(),
      release: os.release(),
      architecture: os.arch(),
      cpuModel: cpus[0]?.model ?? "unknown",
      logicalCpuCount: cpus.length,
      totalMemoryBytes: os.totalmem(),
    },
    git: {
      commit: readGit(["rev-parse", "HEAD"]),
      dirty: status === "unavailable" ? null : status.length > 0,
    },
    datasetExpectation:
      "small or benchmark deterministic synthetic seed; this script verifies that at least one contact row is available",
  };
}

function writeArtifact(output, document) {
  mkdirSync(path.dirname(output), { recursive: true });
  writeFileSync(output, `${JSON.stringify(document, null, 2)}\n`, "utf8");
}

function budgetAssessment(result) {
  const activeDomNodes = Math.max(
    result.routes.dashboard.activeDomNodes,
    result.routes.contacts.activeDomNodes,
    result.finalPage.activeDomNodes,
  );
  const checks = {
    interactionLatency: {
      metric: "controlled end-to-end interaction latency",
      actual: result.interaction.latencyMs,
      unit: "ms",
      target: "<= 150",
      preferred: false,
      met: result.interaction.latencyMs <= 150,
    },
    activeDom: {
      metric: "maximum active DOM nodes in the measured route snapshots",
      actual: activeDomNodes,
      unit: "nodes",
      target: "<= 1500",
      preferred: true,
      met: activeDomNodes <= 1500,
    },
    scrolling: {
      metric: "approximate contacts-grid scrolling rate",
      actual: result.scrolling.approximateFps,
      unit: "frames/second",
      target: ">= 55",
      preferred: false,
      met: result.scrolling.approximateFps >= 55,
    },
    browserHeap: {
      metric: "forced-GC JavaScript heap after the standard scenario",
      actual: result.memory.afterForcedGc.usedBytes,
      unit: "bytes",
      target: "<= 209715200",
      preferred: true,
      met: result.memory.afterForcedGc.usedBytes <= 200 * 1024 * 1024,
    },
    retainedHeapGrowth: {
      metric: "forced-GC used-heap growth approximation",
      actual: result.memory.usedHeapGrowthPercent,
      unit: "percent",
      target: "<= 15",
      preferred: false,
      met:
        result.memory.usedHeapGrowthPercent !== null &&
        result.memory.usedHeapGrowthPercent <= 15,
    },
  };
  return {
    checks,
    allMet: Object.values(checks).every((check) => check.met),
    note: "Preferred targets and hard acceptance goals are both evaluated here; method qualifications in the artifact remain part of every result.",
  };
}

export async function run(options) {
  const startedAt = new Date().toISOString();
  const result = {
    schemaVersion: 1,
    status: "failed",
    generatedAt: startedAt,
    metadata: null,
    methodology: {
      network: "Chrome DevTools Protocol Network events and encodedDataLength",
      memory: "forced-GC Runtime.getHeapUsage approximation",
      interaction:
        "controlled search submit through response, render, and two animation frames",
      scrolling:
        "requestAnimationFrame sampling during a controlled contacts-grid document scroll",
    },
    routes: {},
    interaction: null,
    scrolling: null,
    memory: null,
    finalPage: null,
    budgetAssessment: null,
    browserErrors: [],
  };

  let browser;
  try {
    const [{ chromium }, playwrightPackage] = await Promise.all([
      import("@playwright/test"),
      Promise.resolve(require("@playwright/test/package.json")),
    ]);
    browser = await chromium.launch({
      headless: !options.headed,
      args: [
        "--enable-precise-memory-info",
        "--disable-background-timer-throttling",
        "--disable-renderer-backgrounding",
      ],
    });
    result.metadata = metadata(
      browser.version(),
      playwrightPackage.version,
      options,
      startedAt,
    );
    const context = await browser.newContext({
      viewport: options.viewport,
      locale: "en-US",
      timezoneId: "UTC",
      colorScheme: "light",
      serviceWorkers: "block",
    });
    const page = await context.newPage();
    page.setDefaultTimeout(30_000);
    page.setDefaultNavigationTimeout(30_000);
    page.on("pageerror", (error) => {
      result.browserErrors.push(
        `pageerror: ${sanitizedConsoleMessage(error.message)}`,
      );
    });
    page.on("console", (message) => {
      if (message.type() === "error") {
        result.browserErrors.push(
          `console: ${sanitizedConsoleMessage(message.text())}`,
        );
      }
    });

    const client = await context.newCDPSession(page);
    await Promise.all([
      client.send("Network.enable"),
      client.send("Runtime.enable"),
      client.send("Performance.enable"),
      client.send("HeapProfiler.enable"),
    ]);
    const network = createNetworkTracker(client);

    await page.goto(`${options.baseURL}/login`, {
      waitUntil: "domcontentloaded",
    });
    await page.locator('input[autocomplete="username"]').fill(options.email);
    await page
      .locator('input[autocomplete="current-password"]')
      .fill(options.password);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(/\/dashboard$/, { timeout: 30_000 });
    await waitForAppReady(page, options.settleMs);
    result.metadata.applicationLocale = await ensureEnglishApplicationLocale(
      page,
      options.baseURL,
      options.settleMs,
    );

    result.routes.dashboard = await measureRoute({
      page,
      client,
      network,
      baseURL: options.baseURL,
      route: "/dashboard",
      apiCollection: "dashboard",
      settleMs: options.settleMs,
    });
    result.routes.contacts = await measureRoute({
      page,
      client,
      network,
      baseURL: options.baseURL,
      route: "/contacts",
      apiCollection: "contacts",
      settleMs: options.settleMs,
    });
    await waitForContactRows(page);
    result.interaction = await runSearchInteraction(page, options.settleMs);
    result.scrolling = summarizeScroll(
      await measureScroll(page, options.scrollMs),
    );
    result.memory = await runNavigationCycles(
      page,
      client,
      options.cycles,
      options.settleMs,
    );
    result.finalPage = await pageSnapshot(page, client);
    result.budgetAssessment = budgetAssessment(result);
    if (result.browserErrors.length > 0) {
      throw new Error(
        `browser emitted ${result.browserErrors.length} console or page error(s)`,
      );
    }
    result.status = "passed";
    result.completedAt = new Date().toISOString();
    result.generatedAt = result.completedAt;
    writeArtifact(options.output, result);
    return result;
  } catch (error) {
    result.completedAt = new Date().toISOString();
    result.generatedAt = result.completedAt;
    result.error = {
      name: error instanceof Error ? error.name : "Error",
      message: sanitizedConsoleMessage(
        error instanceof Error ? error.message : String(error),
      ),
    };
    writeArtifact(options.output, result);
    throw error;
  } finally {
    await browser?.close();
  }
}

const invokedPath = process.argv[1]
  ? pathToFileURL(path.resolve(process.argv[1])).href
  : "";
if (invokedPath === import.meta.url) {
  try {
    const options = parseArguments(process.argv.slice(2));
    if (options.help) {
      process.stdout.write(helpText);
    } else {
      const result = await run(options);
      process.stdout.write(
        `Browser performance artifact: ${path.relative(repositoryRoot, options.output)} (${result.status})\n`,
      );
    }
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exitCode = 1;
  }
}
