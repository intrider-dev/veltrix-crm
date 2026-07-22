import { readFile, readdir } from 'node:fs/promises';
import { extname, join, relative, resolve } from 'node:path';
import process from 'node:process';

const root = resolve(import.meta.dirname, '..');
const sourceCatalog = join(root, 'packages/i18n/source/en');
const known = new Set();
for (const file of (await readdir(sourceCatalog)).filter((name) => name.endsWith('.json'))) {
  const namespace = file.slice(0, -5);
  const values = JSON.parse(await readFile(join(sourceCatalog, file), 'utf8'));
  for (const key of Object.keys(values)) known.add(`${namespace}.${key}`);
}

async function sourceFiles(path) {
  const entries = await readdir(path, { withFileTypes: true });
  const result = [];
  for (const entry of entries) {
    const child = join(path, entry.name);
    if (entry.isDirectory()) result.push(...(await sourceFiles(child)));
    else if (['.ts', '.html'].includes(extname(entry.name))) result.push(child);
  }
  return result;
}

const used = new Set();
const unknown = [];
for (const file of await sourceFiles(join(root, 'apps/web/src'))) {
  const contents = await readFile(file, 'utf8');
  for (const match of contents.matchAll(/(?:\.t|\bt)\(\s*['"]([a-z][a-z0-9_.-]+)['"]/g)) {
    used.add(match[1]);
    if (!known.has(match[1])) unknown.push(`${relative(root, file)}: ${match[1]}`);
  }
}
if (unknown.length > 0) {
  process.stderr.write(`Unknown translation keys:\n${unknown.join('\n')}\n`);
  process.exit(1);
}
const unused = [...known].filter((key) => !used.has(key)).sort();
process.stdout.write(`i18n extraction check: ${used.size} used keys, ${unused.length} catalog keys not statically referenced.\n`);
