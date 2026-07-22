import { spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const require = createRequire(import.meta.url);
const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));
const lighthouseVersion = "13.4.1";
const defaultOutputDirectory = path.join(
  repositoryRoot,
  "benchmarks",
  "results",
  "lighthouse",
);

const helpText = `VeltrixCRM Lighthouse runner

Usage:
  node scripts/run-lighthouse.mjs [options]

Options:
  --base-url <url>       Production-like application URL
  --path <route>         Public route to audit (default: /login)
  --output-dir <path>    Artifact directory (default: benchmarks/results/lighthouse)
  --chrome-path <path>   Override local Chrome/Chromium executable via CHROME_PATH
  --profile <value>      desktop, mobile, or all (default: all)
  --dry-run              Print exact pinned commands without invoking Lighthouse
  --help                 Print this message

Environment equivalents:
  LIGHTHOUSE_BASE_URL, BROWSER_BENCHMARK_BASE_URL, E2E_BASE_URL,
  LIGHTHOUSE_PATH, LIGHTHOUSE_OUTPUT_DIR, LIGHTHOUSE_CHROME_PATH, CHROME_PATH

The runner executes lighthouse@13.4.1 through the repository-pinned pnpm CLI and
writes both JSON and HTML for each selected profile.
`;

function normalizedBaseURL(value) {
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

export function parseArguments(argv, environment = process.env) {
  const options = {
    baseURL:
      environment.LIGHTHOUSE_BASE_URL ??
      environment.BROWSER_BENCHMARK_BASE_URL ??
      environment.E2E_BASE_URL ??
      "http://127.0.0.1:8080",
    route: environment.LIGHTHOUSE_PATH ?? "/login",
    outputDirectory:
      environment.LIGHTHOUSE_OUTPUT_DIR ?? defaultOutputDirectory,
    chromePath:
      environment.LIGHTHOUSE_CHROME_PATH ?? environment.CHROME_PATH ?? "",
    profile: "all",
    dryRun: false,
    help: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--") continue;
    if (argument === "--help" || argument === "-h") options.help = true;
    else if (argument === "--dry-run") options.dryRun = true;
    else {
      const value = argv[index + 1];
      if (!value || value.startsWith("--"))
        throw new Error(`${argument} requires a value`);
      index += 1;
      switch (argument) {
        case "--base-url":
          options.baseURL = value;
          break;
        case "--path":
          options.route = value;
          break;
        case "--output-dir":
          options.outputDirectory = value;
          break;
        case "--chrome-path":
          options.chromePath = value;
          break;
        case "--profile":
          options.profile = value;
          break;
        default:
          throw new Error(`unknown option: ${argument}`);
      }
    }
  }

  if (!["desktop", "mobile", "all"].includes(options.profile)) {
    throw new Error("profile must be desktop, mobile, or all");
  }
  const baseURL = normalizedBaseURL(options.baseURL);
  const target = new URL(options.route, `${baseURL}/`);
  if (target.origin !== new URL(baseURL).origin) {
    throw new Error(
      "Lighthouse path must resolve on the configured base URL origin",
    );
  }
  return {
    ...options,
    baseURL,
    route: `${target.pathname}${target.search}`,
    targetURL: target.href,
    outputDirectory: path.resolve(repositoryRoot, options.outputDirectory),
  };
}

function pnpmInvocation() {
  const configured = process.env.npm_execpath;
  if (configured && /pnpm/i.test(configured) && existsSync(configured)) {
    return { command: process.execPath, prefix: [configured] };
  }

  const packagePath = require.resolve("pnpm");
  const packageDocument = JSON.parse(readFileSync(packagePath, "utf8"));
  const relativeBinary =
    typeof packageDocument.bin === "string"
      ? packageDocument.bin
      : packageDocument.bin?.pnpm;
  if (!relativeBinary)
    throw new Error("the installed pnpm package does not expose its CLI path");
  return {
    command: process.execPath,
    prefix: [path.resolve(path.dirname(packagePath), relativeBinary)],
  };
}

function profileArguments(profile, targetURL, outputPrefix) {
  const common = [
    "dlx",
    `lighthouse@${lighthouseVersion}`,
    targetURL,
    "--quiet",
    "--output=json",
    "--output=html",
    `--output-path=${outputPrefix}`,
    "--only-categories=performance,accessibility",
    "--locale=en-US",
    "--throttling-method=simulate",
    "--chrome-flags=--headless=new --disable-gpu --no-first-run --disable-dev-shm-usage",
  ];
  if (profile === "desktop") return [...common, "--preset=desktop"];
  return [
    ...common,
    "--form-factor=mobile",
    "--screenEmulation.mobile=true",
    "--screenEmulation.width=390",
    "--screenEmulation.height=844",
    "--screenEmulation.deviceScaleFactor=2",
  ];
}

function expectedArtifacts(outputPrefix) {
  return {
    json: `${outputPrefix}.report.json`,
    html: `${outputPrefix}.report.html`,
  };
}

function hasCompleteReport(artifacts) {
  if (!existsSync(artifacts.json) || !existsSync(artifacts.html)) return false;
  try {
    const document = JSON.parse(readFileSync(artifacts.json, "utf8"));
    return Boolean(
      document.lighthouseVersion &&
        document.categories?.performance &&
        document.categories?.accessibility,
    );
  } catch {
    return false;
  }
}

function score(value) {
  return typeof value === "number" ? Math.round(value * 100) : null;
}

function auditValue(document, id) {
  const value = document.audits?.[id]?.numericValue;
  return typeof value === "number" ? Math.round(value * 100) / 100 : null;
}

function manifestEntry(profile, artifacts) {
  const document = JSON.parse(readFileSync(artifacts.json, "utf8"));
  return {
    profile,
    requestedURL: document.requestedUrl ?? null,
    finalURL: document.finalDisplayedUrl ?? document.finalUrl ?? null,
    fetchTime: document.fetchTime ?? null,
    lighthouseVersion: document.lighthouseVersion ?? null,
    userAgent: document.userAgent ?? null,
    scores: {
      performance: score(document.categories?.performance?.score),
      accessibility: score(document.categories?.accessibility?.score),
    },
    audits: {
      largestContentfulPaintMs: auditValue(
        document,
        "largest-contentful-paint",
      ),
      cumulativeLayoutShift: auditValue(document, "cumulative-layout-shift"),
      totalBlockingTimeMs: auditValue(document, "total-blocking-time"),
    },
    files: {
      json: path.relative(repositoryRoot, artifacts.json).replaceAll("\\", "/"),
      html: path.relative(repositoryRoot, artifacts.html).replaceAll("\\", "/"),
    },
  };
}

function commandForDisplay(command, args) {
  return [command, ...args]
    .map((argument) =>
      /\s/.test(argument) ? JSON.stringify(argument) : argument,
    )
    .join(" ");
}

export function run(options) {
  const profiles =
    options.profile === "all" ? ["desktop", "mobile"] : [options.profile];
  const invocation = pnpmInvocation();
  const plans = profiles.map((profile) => {
    const outputPrefix = path.join(options.outputDirectory, profile);
    return {
      profile,
      outputPrefix,
      artifacts: expectedArtifacts(outputPrefix),
      args: [
        ...invocation.prefix,
        ...profileArguments(profile, options.targetURL, outputPrefix),
      ],
    };
  });

  if (options.dryRun) {
    process.stdout.write(
      `${JSON.stringify(
        {
          lighthouseVersion,
          targetURL: options.targetURL,
          chromePath: options.chromePath || "automatic Chrome discovery",
          commands: plans.map((plan) =>
            commandForDisplay(invocation.command, plan.args),
          ),
          expectedArtifacts: plans.map((plan) => plan.artifacts),
        },
        null,
        2,
      )}\n`,
    );
    return null;
  }

  mkdirSync(options.outputDirectory, { recursive: true });
  const childEnvironment = { ...process.env };
  if (options.chromePath) childEnvironment.CHROME_PATH = options.chromePath;

  for (const plan of plans) {
    const child = spawnSync(invocation.command, plan.args, {
      cwd: repositoryRoot,
      env: childEnvironment,
      stdio: "inherit",
    });
    if (child.error) throw child.error;
    if (child.status !== 0 && !hasCompleteReport(plan.artifacts)) {
      throw new Error(
        `Lighthouse ${plan.profile} exited with code ${child.status ?? "unknown"}`,
      );
    }
    if (child.status !== 0) {
      process.stderr.write(
        `Lighthouse ${plan.profile} returned ${child.status}, but produced a complete report; ` +
          `continuing because Chrome cleanup can fail with EPERM on Windows.\n`,
      );
    }
    for (const artifact of Object.values(plan.artifacts)) {
      if (!existsSync(artifact))
        throw new Error(`Lighthouse did not create ${artifact}`);
    }
  }

  const manifest = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    targetURL: options.targetURL,
    invokedPackage: `lighthouse@${lighthouseVersion}`,
    host: {
      platform: os.platform(),
      release: os.release(),
      architecture: os.arch(),
    },
    profiles: plans.map((plan) => manifestEntry(plan.profile, plan.artifacts)),
  };
  const manifestPath = path.join(options.outputDirectory, "manifest.json");
  writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, "utf8");
  return manifestPath;
}

const invokedPath = process.argv[1]
  ? pathToFileURL(path.resolve(process.argv[1])).href
  : "";
if (invokedPath === import.meta.url) {
  try {
    const options = parseArguments(process.argv.slice(2));
    if (options.help) process.stdout.write(helpText);
    else {
      const manifestPath = run(options);
      if (manifestPath) {
        process.stdout.write(
          `Lighthouse artifacts: ${path.relative(repositoryRoot, path.dirname(manifestPath))}\n`,
        );
      }
    }
  } catch (error) {
    process.stderr.write(
      `${error instanceof Error ? error.message : String(error)}\n`,
    );
    process.exitCode = 1;
  }
}
