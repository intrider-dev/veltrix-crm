import { execFileSync } from 'node:child_process';
import { readFile, readdir, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import process from 'node:process';

const root = resolve(import.meta.dirname, '..');
const resultRoot = resolve(root, 'benchmarks/results');
const profile = process.argv[2] ?? 'baseline';
if (!['baseline', 'stretch'].includes(profile)) {
  process.stderr.write('profile must be baseline or stretch\n');
  process.exit(2);
}

const pattern = new RegExp(`^k6-${profile}-run-(\\d+)\\.summary\\.json$`);
const files = (await readdir(resultRoot))
  .map((name) => ({ name, match: name.match(pattern) }))
  .filter((item) => item.match)
  .sort((left, right) => Number(left.match[1]) - Number(right.match[1]));
if (files.length === 0) {
  process.stderr.write(`no ${profile} summary files found in ${resultRoot}\n`);
  process.exit(1);
}

const selectors = {
  throughputPerSecond: ['crm_operations', 'rate'],
  errorRate: ['crm_errors', 'rate'],
  readMedianMs: ['crm_read_latency', 'med'],
  readP95Ms: ['crm_read_latency', 'p(95)'],
  readP99Ms: ['crm_read_latency', 'p(99)'],
  writeMedianMs: ['crm_write_latency', 'med'],
  writeP95Ms: ['crm_write_latency', 'p(95)'],
  writeP99Ms: ['crm_write_latency', 'p(99)'],
  searchMedianMs: ['crm_search_latency', 'med'],
  searchP95Ms: ['crm_search_latency', 'p(95)'],
  searchP99Ms: ['crm_search_latency', 'p(99)'],
  overallP95Ms: ['http_req_duration', 'p(95)'],
  overallP99Ms: ['http_req_duration', 'p(99)'],
  transferredBytes: ['data_received', 'count'],
};

const runs = [];
for (const file of files) {
  const document = JSON.parse(await readFile(resolve(resultRoot, file.name), 'utf8'));
  const values = {};
  for (const [name, [metric, statistic]] of Object.entries(selectors)) {
    const value = document.metrics?.[metric]?.values?.[statistic];
    values[name] = Number.isFinite(value) ? value : null;
  }
  const run = Number(file.match[1]);
  const resourcePeaks = await readResourcePeaks(
    resolve(resultRoot, `docker-stats-${profile}-run-${run}.jsonl`),
  );
  runs.push({ run, source: file.name, ...values, resourcePeaks });
}

const median = {};
for (const name of Object.keys(selectors)) {
  const values = runs.map((run) => run[name]).filter(Number.isFinite).sort((a, b) => a - b);
  median[name] = values.length === 0
    ? null
    : values.length % 2 === 1
      ? values[(values.length - 1) / 2]
      : (values[values.length / 2 - 1] + values[values.length / 2]) / 2;
}

const resourcePeakMedian = {};
for (const service of ['app', 'postgres']) {
  resourcePeakMedian[service] = {};
  for (const metric of ['maxCpuPercent', 'maxMemoryBytes', 'maxPids']) {
    const values = runs
      .map((run) => run.resourcePeaks?.[service]?.[metric])
      .filter(Number.isFinite)
      .sort((a, b) => a - b);
    resourcePeakMedian[service][metric] = values.length === 0
      ? null
      : values.length % 2 === 1
        ? values[(values.length - 1) / 2]
        : (values[values.length / 2 - 1] + values[values.length / 2]) / 2;
  }
}

let commit = 'uncommitted';
try {
  commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim();
} catch {
  // A first local benchmark may run before the initial commit.
}

const report = {
  generatedAt: new Date().toISOString(),
  commit,
  profile,
  datasetProfile: 'benchmark',
  runs: runs.length,
  aggregation: 'median of independent clean-dataset runs',
  median,
  resourcePeakMedian,
  individualRuns: runs,
};
const output = resolve(resultRoot, `k6-${profile}-summary.json`);
await writeFile(output, `${JSON.stringify(report, null, 2)}\n`, 'utf8');
process.stdout.write(`wrote ${output} from ${runs.length} independent run(s)\n`);

async function readResourcePeaks(path) {
  const peaks = {};
  let contents;
  try {
    contents = await readFile(path, 'utf8');
  } catch {
    return peaks;
  }
  for (const line of contents.split(/\r?\n/)) {
    if (!line.trim().startsWith('{')) continue;
    let value;
    try {
      value = JSON.parse(line);
    } catch {
      continue;
    }
    const name = String(value.Name ?? value.Container ?? '');
    const service = name.endsWith('-app-1')
      ? 'app'
      : name.endsWith('-postgres-1')
        ? 'postgres'
        : null;
    if (!service) continue;
    const current = peaks[service] ?? { maxCpuPercent: 0, maxMemoryBytes: 0, maxPids: 0 };
    current.maxCpuPercent = Math.max(current.maxCpuPercent, parseFloat(value.CPUPerc) || 0);
    current.maxMemoryBytes = Math.max(
      current.maxMemoryBytes,
      parseSize(String(value.MemUsage ?? '').split('/')[0]?.trim()),
    );
    current.maxPids = Math.max(current.maxPids, Number.parseInt(value.PIDs, 10) || 0);
    peaks[service] = current;
  }
  return peaks;
}

function parseSize(value) {
  const match = value.match(/^([0-9.]+)\s*([kmgt]?i?b)$/i);
  if (!match) return 0;
  const unit = match[2].toLowerCase();
  const powers = { b: 0, kb: 1, kib: 1, mb: 2, mib: 2, gb: 3, gib: 3, tb: 4, tib: 4 };
  const base = unit.includes('i') ? 1024 : 1000;
  return Number(match[1]) * base ** powers[unit];
}
