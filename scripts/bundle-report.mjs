import { brotliCompressSync, constants, gzipSync } from "node:zlib";
import { mkdir, readFile, readdir, stat, writeFile } from "node:fs/promises";
import { basename, extname, join, relative, resolve } from "node:path";
import process from "node:process";

const root = resolve(import.meta.dirname, "..");
const dist = resolve(root, process.env.WEB_DIST ?? "apps/web/dist/web/browser");
const output = resolve(root, "benchmarks/results/bundle-report.json");
const initialTarget = 350 * 1024;
const initialWarning = 400 * 1024;
const initialFailure = 450 * 1024;
const lazyTarget = 200 * 1024;

async function files(path) {
  const entries = await readdir(path, { withFileTypes: true });
  const result = [];
  for (const entry of entries) {
    const child = join(path, entry.name);
    if (entry.isDirectory()) result.push(...(await files(child)));
    else result.push(child);
  }
  return result;
}

let index;
try {
  index = await readFile(join(dist, "index.html"), "utf8");
} catch (error) {
  process.stderr.write(
    `Bundle report requires ${join(dist, "index.html")}: ${error.message}\n`,
  );
  process.exit(1);
}

const initialNames = new Set();
for (const match of index.matchAll(
  /(?:src|href)=["']([^"']+\.(?:js|css))["']/g,
)) {
  initialNames.add(basename(match[1].split("?")[0]));
}

const inlineEventHandlers = [
  ...index.matchAll(/\son[a-z][a-z0-9_-]*\s*=/gi),
].map((match) => match[0].trim().split("=")[0]);

const assets = [];
for (const file of await files(dist)) {
  if (![".js", ".css"].includes(extname(file))) continue;
  const contents = await readFile(file);
  const name = relative(dist, file).replaceAll("\\", "/");
  const lowered = contents.toString("utf8").toLowerCase();
  assets.push({
    name,
    initial: initialNames.has(basename(file)),
    category: lowered.includes("ag-grid")
      ? "ag-grid"
      : lowered.includes("reports")
        ? "reports"
        : "feature",
    rawBytes: contents.byteLength,
    gzipBytes: gzipSync(contents, { level: 9, mtime: 0 }).byteLength,
    brotliBytes: brotliCompressSync(contents, {
      params: {
        [constants.BROTLI_PARAM_QUALITY]: 11,
        [constants.BROTLI_PARAM_MODE]: constants.BROTLI_MODE_TEXT,
      },
    }).byteLength,
  });
}
assets.sort((left, right) => left.name.localeCompare(right.name));
const initial = assets.filter((asset) => asset.initial);
const initialBrotliBytes = initial.reduce(
  (sum, asset) => sum + asset.brotliBytes,
  0,
);
const lazyWarnings = assets.filter(
  (asset) =>
    !asset.initial &&
    asset.category === "feature" &&
    asset.brotliBytes > lazyTarget,
);

const externalFontPattern = /https?:\/\/[^"')\s]*(?:font|typeface)/i;
const textAssets = (await files(dist)).filter((file) =>
  [".html", ".css", ".js"].includes(extname(file)),
);
const externalFontRequests = [];
for (const file of textAssets) {
  const contents = await readFile(file, "utf8");
  if (externalFontPattern.test(contents))
    externalFontRequests.push(relative(dist, file).replaceAll("\\", "/"));
}

const report = {
  generatedAt: new Date().toISOString(),
  method:
    "Node zlib gzip level 9 and Brotli quality 11 over emitted JS/CSS assets",
  budgets: { initialTarget, initialWarning, initialFailure, lazyTarget },
  initialBrotliBytes,
  initialStatus:
    initialBrotliBytes > initialFailure
      ? "failure"
      : initialBrotliBytes > initialWarning
        ? "warning"
        : initialBrotliBytes <= initialTarget
          ? "target-met"
          : "target-missed",
  externalFontRequests,
  inlineEventHandlers,
  lazyWarnings: lazyWarnings.map((asset) => asset.name),
  assets,
};
await mkdir(resolve(root, "benchmarks/results"), { recursive: true });
await writeFile(output, `${JSON.stringify(report, null, 2)}\n`, "utf8");
process.stdout.write(
  `Initial JS+CSS: ${(initialBrotliBytes / 1024).toFixed(1)} KiB Brotli (${report.initialStatus})\n`,
);
for (const asset of assets.filter((item) => !item.initial)) {
  process.stdout.write(
    `Lazy ${asset.name}: ${(asset.brotliBytes / 1024).toFixed(1)} KiB Brotli [${asset.category}]\n`,
  );
}
if (externalFontRequests.length > 0) {
  process.stderr.write(
    `External font references found in: ${externalFontRequests.join(", ")}\n`,
  );
  process.exit(1);
}
if (inlineEventHandlers.length > 0) {
  process.stderr.write(
    `CSP-incompatible inline event handlers found in index.html: ${inlineEventHandlers.join(", ")}\n`,
  );
  process.exit(1);
}
if (lazyWarnings.length > 0) {
  process.stderr.write(
    `Lazy feature target exceeded: ${lazyWarnings.map((asset) => asset.name).join(", ")}\n`,
  );
}
if (initialBrotliBytes > initialFailure) process.exit(1);
