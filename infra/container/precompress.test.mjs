import assert from "node:assert/strict";
import { brotliDecompress, gunzip } from "node:zlib";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import test from "node:test";

import { precompress } from "./precompress.mjs";

const decompressBrotli = promisify(brotliDecompress);
const decompressGzip = promisify(gunzip);

test("precompress creates smaller deterministic representations", async () => {
  const fixture = await mkdtemp(join(tmpdir(), "crm-precompress-"));
  try {
    const source = Buffer.from('const locale = "en";\n'.repeat(512));
    await writeFile(join(fixture, "main-ABCDEF12.js"), source);
    await writeFile(join(fixture, "tiny.json"), "{}");
    await writeFile(join(fixture, "already.png"), source);

    const result = await precompress(fixture);
    assert.equal(result.assetCount, 1);
    assert.equal(result.originalBytes, source.byteLength);
    assert.ok(result.brotliBytes > 0 && result.brotliBytes < source.byteLength);
    assert.ok(result.gzipBytes > 0 && result.gzipBytes < source.byteLength);

    const [brotliSource, gzipSource] = await Promise.all([
      decompressBrotli(await readFile(join(fixture, "main-ABCDEF12.js.br"))),
      decompressGzip(await readFile(join(fixture, "main-ABCDEF12.js.gz"))),
    ]);
    assert.deepEqual(brotliSource, source);
    assert.deepEqual(gzipSource, source);
    await assert.rejects(readFile(join(fixture, "tiny.json.br")));
    await assert.rejects(readFile(join(fixture, "already.png.br")));
  } finally {
    await rm(fixture, { recursive: true, force: true });
  }
});
