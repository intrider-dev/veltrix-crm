import { expect, test } from "@playwright/test";

import {
  createContact,
  currentSession,
  failOnBrowserErrors,
  loginAsDemo,
  scenarioSuffix,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("an enabled automation creates a localized notification and auditable event", async ({
  page,
}, testInfo) => {
  test.slow();
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/automations");
  await waitForAppReady(page);
  const suffix = scenarioSuffix(testInfo.project.name);
  const session = await currentSession(page);
  const ruleName = `Notify contact owner ${suffix}`;
  const notificationTitle = `Automation ${suffix}`;

  await page.getByRole("button", { name: "Add rule", exact: true }).click();
  const editor = page.locator("form.feature-form");
  await editor.getByLabel("Name", { exact: true }).fill(ruleName);
  await page
    .getByRole("combobox", { name: "Trigger", exact: true })
    .selectOption("record_created");
  await page
    .getByRole("combobox", { name: "Record type", exact: true })
    .selectOption("contact");
  await editor.getByLabel("Condition field", { exact: true }).fill("status");
  await page
    .getByRole("combobox", { name: "Comparator", exact: true })
    .selectOption("equals");
  await editor.getByLabel("Condition value", { exact: true }).fill("active");
  await page
    .getByRole("combobox", { name: "Action", exact: true })
    .selectOption("create_notification");
  await editor.getByLabel("Action parameters (JSON)", { exact: true }).fill(
    JSON.stringify({
      recipientId: session.user.id,
      messageKey: "notifications.activity.assigned",
      messageParams: { title: notificationTitle },
    }),
  );
  await editor.getByRole("button", { name: "Create", exact: true }).click();

  const rule = page
    .locator(".record-list article")
    .filter({ hasText: ruleName });
  await expect(rule).toBeVisible();
  const enableResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().includes("/automations/") &&
      response.url().endsWith("/enabled"),
  );
  await rule.getByRole("button", { name: "Disabled", exact: true }).click();
  expect((await enableResponsePromise).status()).toBe(200);
  await expect(
    rule.getByRole("button", { name: "Enabled", exact: true }),
  ).toBeVisible();

  const contactId = await createContact(
    page,
    { first: "Automated", last: `Contact ${suffix}` },
    `automated-${suffix}@example.invalid`,
  );
  await page.goto("/notifications");
  await waitForAppReady(page);
  const notification = page
    .locator(".notification-list article")
    .filter({ hasText: `Assigned activity: ${notificationTitle}` });
  await expect(notification).toBeVisible({ timeout: 45_000 });
  await notification
    .getByRole("button", { name: "Mark as read", exact: true })
    .click();
  await expect(
    notification.getByRole("button", { name: "Mark as read", exact: true }),
  ).toHaveCount(0);

  await page.goto("/settings/audit");
  await waitForAppReady(page);
  const auditEvent = page
    .locator(".audit-table article")
    .filter({ hasText: "Contact created" })
    .filter({ hasText: contactId });
  await expect(auditEvent).toBeVisible();
  assertNoBrowserErrors();
});
