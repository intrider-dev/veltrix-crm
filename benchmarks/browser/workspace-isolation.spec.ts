import { expect, test } from "@playwright/test";

import {
  createContact,
  currentWorkspace,
  failOnBrowserErrors,
  loginAsDemo,
  openGlobalSearch,
  scenarioSuffix,
  selectWorkspaceByName,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("workspace creation and switching preserve tenant isolation", async ({
  page,
}, testInfo) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/dashboard");
  await waitForAppReady(page);
  const originalWorkspace = await currentWorkspace(page);

  const suffix = scenarioSuffix(testInfo.project.name);
  const firstName = "Isolation";
  const lastName = `Sentinel ${suffix}`;
  const contactId = await createContact(
    page,
    { first: firstName, last: lastName },
    `isolation-${suffix}@example.invalid`,
  );

  const workspaceName = `Isolated ${suffix}`;
  await page.goto("/workspace/new");
  await waitForAppReady(page);
  await page.getByLabel("Workspace name").fill(workspaceName);
  await page
    .getByLabel("Workspace URL slug")
    .fill(`isolated-${suffix}`.slice(0, 63));
  await page.getByLabel("Workspace timezone").fill("UTC");
  await page.getByLabel("Default currency").fill("USD");
  await page
    .getByRole("button", { name: "Create workspace", exact: true })
    .click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.locator("mat-select.workspace-select")).toContainText(
    workspaceName,
  );

  try {
    const isolatedWorkspace = await currentWorkspace(page);
    const crossTenantRead = await page
      .context()
      .request.get(
        `/api/v1/workspaces/${isolatedWorkspace.id}/contacts/${contactId}`,
      );
    expect(crossTenantRead.status()).toBe(404);

    const palette = await openGlobalSearch(page, `${firstName} ${lastName}`);
    await expect(palette).toContainText("No matching records.");
    await expect(palette.locator(".palette-results a")).toHaveCount(0);
    await palette.getByRole("button", { name: "Close", exact: true }).click();
  } finally {
    await selectWorkspaceByName(page, originalWorkspace.name);
  }
  await page.goto(`/contacts/${contactId}`);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    `${firstName} ${lastName}`,
  );
  assertNoBrowserErrors();
});
