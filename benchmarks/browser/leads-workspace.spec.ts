import { expect, test } from "@playwright/test";

import {
  failOnBrowserErrors,
  loginAsDemo,
  scenarioSuffix,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("lead list, Kanban, details, discussion, and Gantt stay connected", async ({
  page,
}, testInfo) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/leads");
  await waitForAppReady(page);

  await expect(
    page.getByRole("button", { name: "Kanban", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Gantt", exact: true }),
  ).toBeVisible();
  await expect(page.locator("app-error-panel")).toHaveCount(0);

  const suffix = scenarioSuffix(testInfo.project.name);
  const leadName = `Qualified lead ${suffix}`;
  await page.getByRole("button", { name: "Add lead", exact: true }).click();
  const editor = page.locator("section.editor");
  await expect(editor.getByRole("heading", { name: "New lead" })).toBeVisible();
  await editor.getByLabel("Name", { exact: true }).fill(leadName);
  await editor
    .getByLabel("Email", { exact: true })
    .fill(`${suffix}@example.invalid`);
  const createButton = editor.getByRole("button", {
    name: "Create",
    exact: true,
  });
  await expect(createButton).toBeEnabled();
  const createResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/leads"),
  );
  await createButton.click();
  expect((await createResponse).status()).toBe(201);

  const listLink = page.getByRole("link", { name: leadName, exact: true });
  await expect(listLink).toBeVisible();
  const href = await listLink.getAttribute("href");
  expect(href).toMatch(/^\/leads\/[0-9a-f-]+$/);
  const leadId = href?.split("/").at(-1);
  expect(leadId).toMatch(/^[0-9a-f-]+$/);

  await page.getByRole("button", { name: "Kanban", exact: true }).click();
  const stages = page.locator("article.lead-stage");
  await expect.poll(() => stages.count()).toBeGreaterThanOrEqual(2);
  const sourceCard = page
    .locator("article.lead-card")
    .filter({ hasText: leadName });
  await expect(sourceCard).toBeVisible();
  const targetStage = stages.nth(1);
  const targetStageName = (
    await targetStage.getByRole("heading", { level: 2 }).innerText()
  ).trim();
  const stageResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().includes(`/leads/${leadId}/stage`),
  );
  await sourceCard.locator("select").selectOption({ label: targetStageName });
  expect((await stageResponse).status()).toBe(200);
  await expect(
    targetStage.locator("article.lead-card").filter({ hasText: leadName }),
  ).toBeVisible();

  await targetStage.getByRole("link", { name: leadName, exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/leads/${leadId}$`));
  await waitForAppReady(page);
  await page.getByLabel("Planned start").fill("2026-08-03");
  await page.getByLabel("Expected close").fill("2026-08-14");
  const saveResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "PUT" &&
      response.url().endsWith(`/leads/${leadId}`),
  );
  await page.getByRole("button", { name: "Save", exact: true }).click();
  expect((await saveResponse).status()).toBe(200);

  const discussion = page.locator("app-entity-chat");
  await expect(
    discussion.getByRole("heading", { name: "Record discussion" }),
  ).toBeVisible();
  const message = `First record message ${suffix}`;
  await discussion.getByPlaceholder("Message").fill(message);
  const sendButton = discussion.getByRole("button", {
    name: "Send",
    exact: true,
  });
  await expect(sendButton).toBeEnabled();
  const sendResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().includes("/messages"),
  );
  await sendButton.click();
  expect((await sendResponse).status()).toBe(201);
  await expect(
    discussion.locator("article").filter({ hasText: message }),
  ).toBeVisible();

  await page.getByRole("link", { name: "Back to leads", exact: true }).click();
  await waitForAppReady(page);
  await page.getByRole("button", { name: "Gantt", exact: true }).click();
  await expect(page.locator("section.lead-gantt")).toBeVisible();
  await expect(
    page.locator("section.lead-gantt").getByRole("link", { name: leadName }),
  ).toBeVisible();
  await expect(page.locator("app-error-panel")).toHaveCount(0);
  assertNoBrowserErrors();
});
