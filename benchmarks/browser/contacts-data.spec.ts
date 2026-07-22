import { expect, test } from "@playwright/test";

import {
  createCompany,
  failOnBrowserErrors,
  loginAsDemo,
  scenarioSuffix,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("CSV preview, linked import, saved view, and filtered export use real data", async ({
  page,
}, testInfo) => {
  test.slow();
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  const suffix = scenarioSuffix(testInfo.project.name);
  const companyName = `CSV Company ${suffix}`;
  const firstName = "Imported";
  const lastName = `Contact ${suffix}`;
  const displayName = `${firstName} ${lastName}`;
  const email = `import-${suffix}@example.invalid`;

  await createCompany(page, {
    name: companyName,
    domain: `csv-${suffix}.example.invalid`,
    industry: "Synthetic import",
  });

  await page.goto("/contacts");
  await waitForAppReady(page);
  await page.getByRole("button", { name: "Import CSV", exact: true }).click();
  const importDialog = page.getByRole("dialog", { name: "Import contacts" });
  await importDialog.locator('input[type="file"]').evaluate(
    (input, file) => {
      const transfer = new DataTransfer();
      transfer.items.add(
        new File([file.contents], file.name, { type: "text/csv" }),
      );
      (input as HTMLInputElement).files = transfer.files;
      input.dispatchEvent(new Event("change", { bubbles: true }));
    },
    {
      name: `contacts-${suffix}.csv`,
      contents: `firstName,lastName,email,company_name\n${firstName},${lastName},${email},${companyName}\n`,
    },
  );
  await expect(
    importDialog.getByRole("heading", { name: "Column mapping" }),
  ).toBeVisible({
    timeout: 15_000,
  });
  await expect(importDialog).toContainText("1 data rows");
  await importDialog
    .getByRole("button", { name: "Start import", exact: true })
    .click();
  await expect(
    importDialog.getByRole("heading", { name: "Import completed" }),
  ).toBeVisible({
    timeout: 40_000,
  });
  await expect(importDialog).toContainText("Created");
  await expect(
    importDialog.locator("dl div").filter({ hasText: "Created" }).locator("dd"),
  ).toHaveText("1");
  await expect(
    importDialog.locator("dl div").filter({ hasText: "Errors" }).locator("dd"),
  ).toHaveText("0");
  await importDialog
    .getByRole("button", { name: "Close", exact: true })
    .click();

  const search = page.getByLabel("Search contacts");
  await search.fill(displayName);
  await page.getByRole("button", { name: "Search", exact: true }).click();
  const importedRow = page
    .getByRole("grid")
    .getByRole("row")
    .filter({ hasText: displayName });
  await expect(importedRow).toHaveCount(1);
  await importedRow.click();
  await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]+$/);
  await expect(
    page.getByRole("link", { name: companyName, exact: true }),
  ).toBeVisible();

  await page.goto("/contacts");
  await waitForAppReady(page);
  await page.getByLabel("Search contacts").fill(displayName);
  await page.getByRole("button", { name: "Search", exact: true }).click();
  const viewName = `Imported ${suffix}`;
  await page
    .getByRole("button", { name: "Save current filters", exact: true })
    .click();
  await page.getByLabel("View name").fill(viewName);
  await page.getByRole("button", { name: "Save view", exact: true }).click();
  await expect(page.locator(".saved-view-picker select")).toHaveValue(/.+/);

  await page.goto("/dashboard");
  await waitForAppReady(page);
  await page.goto("/contacts");
  await waitForAppReady(page);
  await page
    .locator(".saved-view-picker select")
    .selectOption({ label: viewName });
  await expect(page.getByLabel("Search contacts")).toHaveValue(displayName);
  await expect(
    page.getByRole("grid").getByRole("row").filter({ hasText: displayName }),
  ).toHaveCount(1);

  const downloadPromise = page.waitForEvent("download");
  const exportResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "GET" &&
      response.url().includes("/contacts/export"),
  );
  await page
    .getByRole("button", { name: "Export current view", exact: true })
    .click();
  const [download, exportResponse] = await Promise.all([
    downloadPromise,
    exportResponsePromise,
  ]);
  expect(download.suggestedFilename()).toMatch(
    /^contacts-\d{4}-\d{2}-\d{2}\.csv$/,
  );
  expect(exportResponse.status()).toBe(200);
  const csv = await exportResponse.text();
  expect(csv).toContain(email);
  expect(csv).toContain(companyName);
  assertNoBrowserErrors();
});
