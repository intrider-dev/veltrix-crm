import { spawn } from "node:child_process";
import { cp, mkdir, mkdtemp, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join, resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const apiSource = join(root, "apps", "api");
const webAssets = join(root, "apps", "web", "dist", "web", "browser");
const outputDirectory = join(root, "dist");
const output = join(outputDirectory, "veltrix-crm");
const temporaryRoot = await mkdtemp(join(tmpdir(), "veltrix-crm-build-"));
const temporaryAPI = join(temporaryRoot, "api");

try {
  const assetStat = await stat(join(webAssets, "index.html")).catch(() => null);
  if (!assetStat?.isFile()) {
    throw new Error(
      "Angular production assets are missing; run pnpm build:web first",
    );
  }

  await cp(apiSource, temporaryAPI, { recursive: true });
  const temporaryAssets = join(
    temporaryAPI,
    "internal",
    "platform",
    "webui",
    "assets",
  );
  await rm(temporaryAssets, { recursive: true, force: true });
  await cp(webAssets, temporaryAssets, { recursive: true });
  await mkdir(outputDirectory, { recursive: true });

  await run(
    "go",
    [
      "build",
      "-tags",
      "timetzdata",
      "-trimpath",
      "-ldflags=-s -w -buildid=",
      "-o",
      output,
      "./cmd/server",
    ],
    {
      cwd: temporaryAPI,
      env: { ...process.env, CGO_ENABLED: "0", GOWORK: "off" },
    },
  );

  process.stdout.write(
    `Built ${basename(output)} with the Angular SPA embedded from ${webAssets}\n`,
  );
} finally {
  await rm(temporaryRoot, { recursive: true, force: true });
}

function run(command, args, options) {
  return new Promise((resolvePromise, reject) => {
    const child = spawn(command, args, { ...options, stdio: "inherit" });
    child.once("error", reject);
    child.once("exit", (code, signal) => {
      if (code === 0) {
        resolvePromise();
        return;
      }
      reject(
        new Error(
          `${command} exited with ${code ?? `signal ${signal ?? "unknown"}`}`,
        ),
      );
    });
  });
}
