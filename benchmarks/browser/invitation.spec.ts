import { expect, test } from "@playwright/test";

import {
  currentWorkspace,
  failOnBrowserErrors,
  loginAsDemo,
  scenarioSuffix,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

// Invitation tokens are bearer secrets. This scenario deliberately opts out of
// browser artifacts even when the surrounding suite retains failure traces.
test.use({ trace: "off", screenshot: "off", video: "off" });

test("a development user can accept a one-time workspace invitation", async ({
  context,
  page,
}, testInfo) => {
  test.slow();
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/settings/members");
  await waitForAppReady(page);
  const invitedWorkspace = await currentWorkspace(page);
  const suffix = scenarioSuffix(testInfo.project.name);
  const email = `invited-${suffix}@example.invalid`;
  const password = "Demo123!";

  const inviteForm = page.locator("form").filter({
    has: page.getByRole("heading", { name: "Invite a member", exact: true }),
  });
  await inviteForm.getByLabel("Email", { exact: true }).fill(email);
  await inviteForm.getByLabel("Role").selectOption("sales");
  await inviteForm
    .getByRole("button", { name: "Create invitation", exact: true })
    .click();
  const token = (await page.locator(".secret-panel code").innerText()).trim();
  expect(token.length).toBeGreaterThanOrEqual(40);

  await context.clearCookies();
  await page.goto("/register");
  await page.getByLabel("Display name").fill(`Invited ${suffix}`);
  // The registration field is intentionally labelled "Work email" in English;
  // autocomplete is the stable semantic contract across translated labels.
  await page.locator('input[autocomplete="email"]').fill(email);
  const passwords = page.locator('input[autocomplete="new-password"]');
  await passwords.first().fill(password);
  await passwords.nth(1).fill(password);
  await page
    .getByRole("button", { name: "Create development account", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText(email);
  await page
    .getByRole("link", { name: "Continue to sign in", exact: true })
    .click();
  await page.locator('input[autocomplete="username"]').fill(email);
  await page.locator('input[autocomplete="current-password"]').fill(password);
  await page.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(/\/dashboard$/);

  await page.goto(`/invitations/accept?token=${encodeURIComponent(token)}`);
  await page.evaluate(() =>
    history.replaceState({}, "", "/invitations/accept"),
  );
  await page
    .getByRole("button", { name: "Accept invitation", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("Invitation accepted");
  await page
    .getByRole("button", { name: "Open workspace", exact: true })
    .click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.locator("mat-select.workspace-select")).toContainText(
    invitedWorkspace.name,
  );
  assertNoBrowserErrors();
});
