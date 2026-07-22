import { defineConfig } from "@playwright/test";

import { authStatePath } from "./benchmarks/browser/helpers";

const baseURL =
  process.env["E2E_BASE_URL"] ??
  process.env["PLAYWRIGHT_BASE_URL"] ??
  "http://127.0.0.1:8080";

export default defineConfig({
  testDir: "./benchmarks/browser",
  outputDir: "./benchmarks/results/playwright",
  fullyParallel: false,
  workers: 1,
  retries: process.env["CI"] ? 2 : 0,
  forbidOnly: Boolean(process.env["CI"]),
  reporter: [
    ["list"],
    ["html", { outputFolder: "playwright-report", open: "never" }],
  ],
  expect: { timeout: 8_000 },
  timeout: 45_000,
  use: {
    baseURL,
    locale: "en-US",
    timezoneId: "UTC",
    colorScheme: "light",
    serviceWorkers: "allow",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
      use: { browserName: "chromium", viewport: { width: 1440, height: 900 } },
    },
    {
      name: "desktop-1440",
      testIgnore: [/auth\.setup\.ts/, /invitation\.spec\.ts/],
      dependencies: ["setup"],
      use: {
        browserName: "chromium",
        viewport: { width: 1440, height: 900 },
        storageState: authStatePath,
      },
    },
    {
      name: "tablet-1024",
      testIgnore: [
        /auth\.setup\.ts/,
        /invitation\.spec\.ts/,
        /kanban-drag\.spec\.ts/,
      ],
      dependencies: ["setup"],
      use: {
        browserName: "chromium",
        viewport: { width: 1024, height: 768 },
        storageState: authStatePath,
      },
    },
    {
      name: "mobile-390",
      testIgnore: [
        /auth\.setup\.ts/,
        /invitation\.spec\.ts/,
        /kanban-drag\.spec\.ts/,
        /workspace-isolation\.spec\.ts/,
      ],
      dependencies: ["setup"],
      use: {
        browserName: "chromium",
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
        storageState: authStatePath,
      },
    },
    {
      name: "identity-sensitive",
      testMatch: /invitation\.spec\.ts/,
      dependencies: ["setup"],
      use: {
        browserName: "chromium",
        viewport: { width: 1440, height: 900 },
        storageState: authStatePath,
        trace: "off",
        screenshot: "off",
        video: "off",
      },
    },
  ],
});
