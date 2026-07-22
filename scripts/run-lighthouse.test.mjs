import assert from "node:assert/strict";
import { test } from "node:test";

import { parseArguments } from "./run-lighthouse.mjs";

test("builds a same-origin Lighthouse target with explicit profile", () => {
  const options = parseArguments(
    [
      "--",
      "--base-url",
      "http://127.0.0.1:18081/",
      "--path",
      "/login?source=benchmark",
      "--profile",
      "mobile",
      "--chrome-path",
      "C:/Browser/chrome.exe",
      "--dry-run",
    ],
    {},
  );
  assert.equal(options.baseURL, "http://127.0.0.1:18081");
  assert.equal(
    options.targetURL,
    "http://127.0.0.1:18081/login?source=benchmark",
  );
  assert.equal(options.profile, "mobile");
  assert.equal(options.chromePath, "C:/Browser/chrome.exe");
  assert.equal(options.dryRun, true);
});

test("rejects a cross-origin route and unknown profile", () => {
  assert.throws(
    () =>
      parseArguments(
        [
          "--base-url",
          "https://crm.example.test",
          "--path",
          "https://other.example.test",
        ],
        {},
      ),
    /configured base URL origin/,
  );
  assert.throws(
    () => parseArguments(["--profile", "tablet"], {}),
    /desktop, mobile, or all/,
  );
});
