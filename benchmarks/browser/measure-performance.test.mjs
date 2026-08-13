import assert from "node:assert/strict";
import { test } from "node:test";

import {
  normalizedBaseURL,
  parseArguments,
  parseViewport,
  percentile,
} from "./measure-performance.mjs";

test("normalizes an HTTP base URL without accepting embedded credentials", () => {
  assert.equal(
    normalizedBaseURL("http://127.0.0.1:18081/"),
    "http://127.0.0.1:18081",
  );
  assert.throws(() => normalizedBaseURL("ftp://example.test"), /http or https/);
  assert.throws(
    () => normalizedBaseURL("https://user:secret@example.test"),
    /credentials/,
  );
});

test("validates deterministic viewport bounds", () => {
  assert.deepEqual(parseViewport("1440x900"), { width: 1440, height: 900 });
  assert.throws(() => parseViewport("1440:900"), /WIDTHxHEIGHT/);
  assert.throws(() => parseViewport("200x200"), /between/);
});

test("uses nearest-rank percentiles", () => {
  assert.equal(percentile([], 0.95), 0);
  assert.equal(percentile([50, 10, 40, 20, 30], 0.5), 30);
  assert.equal(percentile([50, 10, 40, 20, 30], 0.95), 50);
});

test("parses CLI options over environment defaults", () => {
  const options = parseArguments(
    [
      "--",
      "--base-url",
      "http://127.0.0.1:19090/",
      "--cycles",
      "7",
      "--viewport",
      "1024x768",
      "--settle-ms",
      "250",
      "--scroll-ms",
      "1200",
      "--run",
      "3",
      "--headed",
    ],
    {
      E2E_EMAIL: "fixture@example.test",
      E2E_PASSWORD: "not-written-to-the-artifact",
    },
  );

  assert.equal(options.baseURL, "http://127.0.0.1:19090");
  assert.equal(options.cycles, 7);
  assert.deepEqual(options.viewport, { width: 1024, height: 768 });
  assert.equal(options.settleMs, 250);
  assert.equal(options.scrollMs, 1200);
  assert.equal(options.run, 3);
  assert.equal(options.headed, true);
});
