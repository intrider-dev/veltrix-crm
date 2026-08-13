import { readFile } from "node:fs/promises";

const reportPath = process.argv[2] ?? "license-report.json";
const report = JSON.parse(await readFile(reportPath, "utf8"));

// These licenses are incompatible with the intended permissive distribution
// policy or impose source/network-use restrictions that need an explicit ADR.
const denied = [
  /(^|\W)AGPL(?:-|$)/i,
  /(^|\W)GPL(?:-|$)/i,
  /(^|\W)SSPL(?:-|$)/i,
  /(^|\W)BUSL(?:-|$)/i,
  /Commons Clause/i,
  /UNLICENSED/i,
  /UNKNOWN/i,
];

const violations = [];
for (const [license, packages] of Object.entries(report)) {
  if (!denied.some((pattern) => pattern.test(license))) continue;
  const names = Array.isArray(packages)
    ? packages.map(
        (entry) =>
          `${entry.name ?? "unknown"}@${(entry.versions ?? []).join(",") || "unknown"}`,
      )
    : ["unresolved package"];
  violations.push(`${license}: ${names.join(", ")}`);
}

if (violations.length > 0) {
  console.error("Production dependency license policy violations:");
  for (const violation of violations) console.error(`- ${violation}`);
  process.exitCode = 1;
} else {
  console.log(
    `Checked ${Object.keys(report).length} production license groups; no denied license was found.`,
  );
}
