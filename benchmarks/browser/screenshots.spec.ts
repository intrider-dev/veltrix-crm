import { expect, test, type Page, type TestInfo } from "@playwright/test";

import {
  failOnBrowserErrors,
  firstGridDataRow,
  loginAsDemo,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("captures real application portfolio views", async ({
  page,
}, testInfo) => {
  test.slow();
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");

  await capture(page, testInfo, "dashboard", "/dashboard", async () => {
    await expect(
      page.getByRole("heading", { name: /^Welcome,/, level: 1 }),
    ).toBeVisible();
    await expect(page.locator(".kpi-grid")).toBeVisible();
  });
  await capture(page, testInfo, "contacts-grid", "/contacts", async () => {
    await expect(firstGridDataRow(page)).toBeVisible();
  });
  await capture(page, testInfo, "leads-workspace", "/leads", async () => {
    await expect(page.locator(".stage-overview")).toBeVisible();
    await expect(page.locator(".filter-panel")).toBeVisible();
  });
  await capture(page, testInfo, "deal-pipeline", "/deals", async () => {
    await expect(page.locator(".deal-card").first()).toBeVisible();
  });
  await capture(page, testInfo, "tasks-workspace", "/activities", async () => {
    await expect(page.locator(".activity-list")).toBeVisible();
    await expect(page.locator(".task-insights")).toBeVisible();
  });
  await capture(page, testInfo, "calendar-workspace", "/calendar", async () => {
    await expect(page.locator(".calendar-surface")).toBeVisible();
    await expect(page.locator(".calendar-insights")).toBeVisible();
  });
  await capture(page, testInfo, "reports", "/reports", async () => {
    await expect(
      page.getByRole("heading", { name: "Reports", level: 1 }),
    ).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "Won" })).toBeVisible();
  });

  await page.goto("/contacts");
  await waitForAppReady(page);
  const firstContact = firstGridDataRow(page);
  await expect(
    firstContact,
    "the deterministic seed must contain a contact",
  ).toBeVisible();
  await firstContact.click();
  await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]+$/);
  await waitForAppReady(page);
  await expect(
    page.getByRole("heading", { name: "Activity timeline" }),
  ).toBeVisible();
  await captureCurrent(page, testInfo, "contact-details-timeline");

  await page.goto("/companies");
  await waitForAppReady(page);
  const firstCompany = page.locator(".company-list > a").first();
  await expect(
    firstCompany,
    "the deterministic seed must contain a company",
  ).toBeVisible();
  await captureCurrent(page, testInfo, "companies-workspace");
  await firstCompany.click();
  await expect(page).toHaveURL(/\/companies\/[0-9a-f-]+$/);
  await waitForAppReady(page);
  await expect(
    page.getByRole("heading", { name: "Activity timeline" }),
  ).toBeVisible();
  await captureCurrent(page, testInfo, "company-details-timeline");

  await page.goto("/settings");
  await waitForAppReady(page);
  await page.getByRole("button", { name: "Dark", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await capture(page, testInfo, "dashboard-dark", "/dashboard", async () => {
    await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
    await expect(page.locator(".kpi-grid")).toBeVisible();
  });
  assertNoBrowserErrors();
});

async function capture(
  page: Page,
  testInfo: TestInfo,
  name: string,
  route: string,
  assertContent: () => Promise<void>,
): Promise<void> {
  await page.goto(route);
  await waitForAppReady(page);
  await assertContent();
  await captureCurrent(page, testInfo, name);
}

async function captureCurrent(
  page: Page,
  testInfo: TestInfo,
  name: string,
): Promise<void> {
  await page.screenshot({
    path: testInfo.outputPath(`${name}-${testInfo.project.name}.png`),
    fullPage: true,
    animations: "disabled",
  });
}
