import { promisify } from "node:util";
import { brotliCompress, constants, gzip } from "node:zlib";
import { readdir, readFile, stat, writeFile } from "node:fs/promises";
import { extname, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const brotli = promisify(brotliCompress);
const gzipAsync = promisify(gzip);
const compressible = new Set([
  ".css",
  ".html",
  ".js",
  ".json",
  ".mjs",
  ".svg",
  ".txt",
  ".wasm",
  ".webmanifest",
  ".xml",
]);

async function listFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(
    entries
      .sort((left, right) => left.name.localeCompare(right.name))
      .map(async (entry) => {
        const fullPath = join(directory, entry.name);
        return entry.isDirectory() ? listFiles(fullPath) : [fullPath];
      }),
  );
  return nested.flat();
}

export async function precompress(input) {
  const root = resolve(input);
  const rootStat = await stat(root);
  if (!rootStat.isDirectory()) {
    throw new Error(`asset path is not a directory: ${root}`);
  }

  let assetCount = 0;
  let originalBytes = 0;
  let brotliBytes = 0;
  let gzipBytes = 0;

  for (const file of await listFiles(root)) {
    if (
      file.endsWith(".br") ||
      file.endsWith(".gz") ||
      !compressible.has(extname(file))
    ) {
      continue;
    }

    const source = await readFile(file);
    // Very small payloads often grow after compression headers and filesystem
    // metadata. They remain available as their identity representation.
    if (source.byteLength < 256) {
      continue;
    }

    const [brotliOutput, gzipOutput] = await Promise.all([
      brotli(source, {
        params: {
          [constants.BROTLI_PARAM_MODE]:
            extname(file) === ".wasm"
              ? constants.BROTLI_MODE_GENERIC
              : constants.BROTLI_MODE_TEXT,
          [constants.BROTLI_PARAM_QUALITY]: 11,
        },
      }),
      gzipAsync(source, { level: 9, mtime: 0 }),
    ]);

    if (brotliOutput.byteLength < source.byteLength) {
      await writeFile(`${file}.br`, brotliOutput);
      brotliBytes += brotliOutput.byteLength;
    }
    if (gzipOutput.byteLength < source.byteLength) {
      await writeFile(`${file}.gz`, gzipOutput);
      gzipBytes += gzipOutput.byteLength;
    }
    assetCount += 1;
    originalBytes += source.byteLength;
  }

  return { assetCount, originalBytes, brotliBytes, gzipBytes };
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  const input = process.argv[2];
  if (!input) {
    throw new Error("usage: node precompress.mjs <asset-directory>");
  }

  process.stdout.write(`${JSON.stringify(await precompress(input))}\n`);
}
