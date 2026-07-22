import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

import {
  failOnBrowserErrors,
  firstGridDataRow,
  loginAsDemo,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

const routes = [
  "/dashboard",
  "/contacts",
  "/companies",
  "/leads",
  "/deals",
  "/activities",
  "/calendar",
  "/automations",
  "/reports",
  "/notifications",
  "/settings",
  "/settings/security",
  "/settings/members",
  "/settings/custom-fields",
  "/settings/api",
  "/settings/webhooks",
  "/settings/audit",
  "/settings/localization",
  "/settings/translations",
  "/workspace/new",
] as const;

test("login page has no WCAG 2.2 A/AA violations", async ({
  context,
  page,
}) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await context.clearCookies();
  await page.goto("/login");
  const results = await scan(page);
  expect(results.violations).toEqual([]);
  assertNoBrowserErrors();
});

for (const route of routes) {
  test(`${route} has no WCAG 2.2 A/AA violations`, async ({ page }) => {
    const assertNoBrowserErrors = failOnBrowserErrors(page);
    await loginAsDemo(page);
    await setAppLanguage(page, "en");
    await page.goto(route);
    await waitForAppReady(page);
    const results = await scan(page);
    expect(results.violations).toEqual([]);
    assertNoBrowserErrors();
  });
}

test("seeded contact, company, and deal details have no WCAG 2.2 A/AA violations", async ({
  page,
}) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");

  await page.goto("/contacts");
  await waitForAppReady(page);
  const contact = firstGridDataRow(page);
  await expect(
    contact,
    "the deterministic seed must contain a contact",
  ).toBeVisible();
  await contact.click();
  await waitForAppReady(page);
  expect((await scan(page)).violations).toEqual([]);

  await page.goto("/companies");
  await waitForAppReady(page);
  const company = page.locator(".company-list > a").first();
  await expect(
    company,
    "the deterministic seed must contain a company",
  ).toBeVisible();
  await company.click();
  await waitForAppReady(page);
  expect((await scan(page)).violations).toEqual([]);

  await page.goto("/deals");
  await waitForAppReady(page);
  const deal = page.locator(".deal-card a").first();
  await expect(
    deal,
    "the deterministic seed must contain a deal",
  ).toBeVisible();
  await deal.click();
  await waitForAppReady(page);
  expect((await scan(page)).violations).toEqual([]);
  assertNoBrowserErrors();
});

function scan(page: Page) {
  return new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
}
