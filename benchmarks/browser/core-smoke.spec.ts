import { expect, test } from "@playwright/test";

import {
  failOnBrowserErrors,
  firstGridDataRow,
  loginAsDemo,
  loginWithCredentials,
  openGlobalSearch,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("credential login establishes the expected cookie-backed session", async ({
  context,
  page,
}) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await context.clearCookies();
  await loginWithCredentials(page);

  const cookies = await context.cookies();
  const session = cookies.find((cookie) => cookie.name.endsWith("_session"));
  const csrf = cookies.find((cookie) => cookie.name === "XSRF-TOKEN");
  expect(session, "session cookie").toBeDefined();
  expect(session?.httpOnly).toBe(true);
  expect(session?.sameSite).toBe("Lax");
  expect(csrf, "CSRF double-submit cookie").toBeDefined();
  expect(csrf?.httpOnly).toBe(false);
  assertNoBrowserErrors();
});

test("global search opens a real seeded record", async ({ page }, testInfo) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/contacts");
  await waitForAppReady(page);

  const row = firstGridDataRow(page);
  await expect(
    row,
    "the deterministic seed must contain contacts",
  ).toBeVisible();
  const candidate = (await row.getByRole("gridcell").nth(1).innerText()).trim();
  expect(candidate, `seeded contact name in ${testInfo.project.name}`).not.toBe(
    "",
  );

  const palette = await openGlobalSearch(page, candidate);
  const result = palette
    .locator(".palette-results a")
    .filter({ hasText: candidate })
    .first();
  await expect(result).toBeVisible({ timeout: 15_000 });
  await result.click();
  await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]+$/);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    candidate,
  );
  assertNoBrowserErrors();
});

test("language and dark-theme preferences apply immediately", async ({
  page,
}) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "ru");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Настройки");
  await page.getByRole("button", { name: "Тёмная", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await setAppLanguage(page, "en");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Settings");
  await page.getByRole("button", { name: "Light", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  assertNoBrowserErrors();
});
