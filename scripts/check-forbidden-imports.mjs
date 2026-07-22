import { readFile, readdir } from "node:fs/promises";
import { extname, join, relative, resolve } from "node:path";
import process from "node:process";

const root = resolve(import.meta.dirname, "..");
const forbidden = [
  ["ag-grid-enterprise", /\bag-grid-enterprise\b/],
  ["zone.js", /(?:from\s+['"]zone\.js|['"]zone\.js['"])/],
  ["Moment.js", /(?:from|require\()\s*['"]moment(?:\/|['"])/],
  ["whole Lodash", /(?:from|require\()\s*['"]lodash['"]\)?/],
  ["Tailwind", /\btailwindcss\b|@tailwind\s/],
  ["remote fonts", /fonts\.(?:googleapis|gstatic)\.com/i],
];
const roots = ["apps/web/src", "apps/web/package.json", "package.json"];
const extensions = new Set([
  ".ts",
  ".html",
  ".scss",
  ".css",
  ".json",
  ".yaml",
  ".yml",
]);
const failures = [];

async function scan(path) {
  const statEntries = await readdir(path, { withFileTypes: true }).catch(
    () => null,
  );
  if (statEntries) {
    for (const entry of statEntries) {
      if (entry.name === "node_modules" || entry.name === "dist") continue;
      await scan(join(path, entry.name));
    }
    return;
  }
  if (!extensions.has(extname(path)) && !path.endsWith("pnpm-lock.yaml"))
    return;
  const content = await readFile(path, "utf8");
  const projectPath = relative(root, path).replaceAll("\\", "/");
  for (const [label, pattern] of forbidden) {
    if (pattern.test(content))
      failures.push(`${projectPath}: forbidden ${label}`);
  }
  if (
    projectPath.startsWith("apps/web/src/") &&
    projectPath !== "apps/web/src/app/shared/a11y/focus-after-render.ts" &&
    /\bafterNextRender\b/.test(content)
  ) {
    failures.push(
      `${projectPath}: use focusAfterNextRender so callbacks receive an explicit Injector`,
    );
  }
}

for (const item of roots) await scan(join(root, item));
const lockfile = await readFile(join(root, "pnpm-lock.yaml"), "utf8");
if (/(?:^|\/)ag-grid-enterprise@/m.test(lockfile)) {
  failures.push("pnpm-lock.yaml: forbidden ag-grid-enterprise package");
}
if (failures.length > 0) {
  process.stderr.write(`${failures.join("\n")}\n`);
  process.exit(1);
}
process.stdout.write("forbidden dependency/import check passed\n");
