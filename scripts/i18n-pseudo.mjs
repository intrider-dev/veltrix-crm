import { mkdir, readFile, readdir, writeFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const source = join(root, 'packages/i18n/source/en');
const target = join(root, 'packages/i18n/generated/pseudo');
const accents = new Map(Object.entries({ a: 'á', e: 'ë', i: 'ï', o: 'ô', u: 'ü', A: 'Á', E: 'Ë', I: 'Ï', O: 'Ô', U: 'Ü' }));

function pseudo(message) {
  const placeholders = [];
  const protectedMessage = message.replace(/\{[^}]+\}/g, (value) => `\u0000${placeholders.push(value) - 1}\u0000`);
  const transformed = [...protectedMessage].map((char) => accents.get(char) ?? char).join('');
  const expanded = `${transformed} ${'·'.repeat(Math.max(2, Math.ceil(message.length * 0.3)))}`;
  return `［${expanded.replace(/\u0000(\d+)\u0000/g, (_, index) => placeholders[Number(index)])}］`;
}

await mkdir(target, { recursive: true });
for (const file of (await readdir(source)).filter((name) => name.endsWith('.json')).sort()) {
  const catalog = JSON.parse(await readFile(join(source, file), 'utf8'));
  const output = Object.fromEntries(Object.entries(catalog).map(([key, value]) => [key, pseudo(value)]));
  await writeFile(join(target, file), `${JSON.stringify(output, null, 2)}\n`, 'utf8');
}
process.stdout.write(`pseudo-locale generated in ${target}\n`);
