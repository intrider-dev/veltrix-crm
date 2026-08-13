import { expect, test } from "@playwright/test";

import {
  createCompany,
  failOnBrowserErrors,
  loginAsDemo,
  scenarioSuffix,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("company, deal, and task lifecycle is usable end to end", async ({
  page,
}, testInfo) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  const suffix = scenarioSuffix(testInfo.project.name);

  const companyName = `Northwind ${suffix}`;
  await createCompany(page, {
    name: companyName,
    domain: `${suffix}.example.invalid`,
    industry: "Synthetic testing",
  });
  await expect(
    page.getByRole("heading", { level: 2, name: "Company profile" }),
  ).toBeVisible();
  const [pageBox, gridBox, editBox, timelineBox] = await Promise.all([
    page.locator(".details-page").boundingBox(),
    page.locator(".details-grid").boundingBox(),
    page.locator(".edit-panel").boundingBox(),
    page.locator(".timeline-panel").boundingBox(),
  ]);
  expect(pageBox, "company details page box").not.toBeNull();
  expect(gridBox, "company details grid box").not.toBeNull();
  expect(editBox, "company edit panel box").not.toBeNull();
  expect(timelineBox, "company timeline panel box").not.toBeNull();
  expect(
    gridBox!.width,
    "company details grid stays within its page track",
  ).toBeLessThanOrEqual(pageBox!.width + 0.5);
  const panelsOverlap =
    Math.min(editBox!.x + editBox!.width, timelineBox!.x + timelineBox!.width) >
      Math.max(editBox!.x, timelineBox!.x) &&
    Math.min(
      editBox!.y + editBox!.height,
      timelineBox!.y + timelineBox!.height,
    ) > Math.max(editBox!.y, timelineBox!.y);
  expect(panelsOverlap, "company edit and timeline panels do not overlap").toBe(
    false,
  );
  await page.getByRole("button", { name: "Add activity", exact: true }).click();
  const companyTimeline = page.locator(".timeline-panel");
  await companyTimeline.getByLabel("Activity type").selectOption("note");
  await companyTimeline.getByLabel("Title").fill(`Company note ${suffix}`);
  await companyTimeline
    .getByLabel("Details")
    .fill("Synthetic acceptance note.");
  await companyTimeline
    .getByRole("button", { name: "Add", exact: true })
    .click();
  await expect(companyTimeline).toContainText(`Company note ${suffix}`);

  await page.goto("/deals");
  await waitForAppReady(page);
  const stages = page.locator("article.stage");
  await expect
    .poll(() => stages.count(), { timeout: 15_000 })
    .toBeGreaterThanOrEqual(2);
  const firstStage = stages.first();
  const secondStage = stages.nth(1);
  const secondStageName = (
    await secondStage.getByRole("heading", { level: 2 }).innerText()
  ).trim();
  const dealName = `Expansion ${suffix}`;
  await page.getByRole("button", { name: "Add deal", exact: true }).click();
  const dealDialog = page.getByRole("dialog", { name: "New deal" });
  await dealDialog.getByLabel("Name", { exact: true }).fill(dealName);
  await dealDialog.getByLabel("Amount", { exact: true }).fill("12500");
  await dealDialog.getByLabel("Currency", { exact: true }).fill("USD");
  await dealDialog.getByRole("button", { name: "Create", exact: true }).click();
  const dealCard = firstStage
    .locator("article.deal-card")
    .filter({ hasText: dealName });
  await expect(dealCard).toBeVisible();
  const moveResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().endsWith("/stage"),
  );
  await dealCard
    .getByLabel("Move deal with keyboard")
    .selectOption({ label: secondStageName });
  expect((await moveResponsePromise).status()).toBe(200);
  await expect(
    secondStage.locator("article.deal-card").filter({ hasText: dealName }),
  ).toBeVisible();

  await page.getByRole("button", { name: "List", exact: true }).click();
  await expect(page.locator("section.deal-list")).toBeVisible();
  await expect(page.locator("section.deal-list")).toContainText(dealName);
  await page.getByRole("button", { name: "Gantt", exact: true }).click();
  await expect(page.locator("section.gantt")).toBeVisible();
  await page.getByRole("button", { name: "Kanban", exact: true }).click();
  await expect(page.locator("section.kanban")).toBeVisible();
  await expect(
    secondStage.locator("article.deal-card").filter({ hasText: dealName }),
  ).toBeVisible();

  await page.goto("/activities");
  await waitForAppReady(page);
  const taskName = `Follow up ${suffix}`;
  await page.getByRole("button", { name: "Create task", exact: true }).click();
  const createPanel = page.locator(".create-panel");
  await createPanel.getByLabel("Activity type").selectOption("task");
  await createPanel.getByLabel("Title").fill(taskName);
  await createPanel
    .getByLabel("Details")
    .fill("Created by the production-like browser acceptance flow.");
  await createPanel.getByLabel("Priority").selectOption("high");
  await createPanel
    .getByRole("button", { name: "Create", exact: true })
    .click();

  const task = page
    .locator(".activity-list article")
    .filter({ hasText: taskName });
  await expect(task).toContainText("Open");
  await task
    .getByRole("button", { name: "Mark complete", exact: true })
    .click();
  await expect(task).toContainText("Completed");

  await page.goto("/calendar");
  await waitForAppReady(page);
  const eventName = `Team event ${suffix}`;
  await page.getByRole("button", { name: "Add event", exact: true }).click();
  const eventEditor = page.locator("section.editor");
  await eventEditor.getByLabel("Title", { exact: true }).fill(eventName);
  await eventEditor
    .getByRole("button", { name: "Create", exact: true })
    .click();
  await expect(
    page.locator(".calendar-event").filter({ hasText: eventName }),
  ).toBeVisible();

  await page.goto("/projects");
  await waitForAppReady(page);
  const projectName = `Launch project ${suffix}`;
  await page.getByRole("button", { name: "New project", exact: true }).click();
  const projectDialog = page.getByRole("dialog", { name: "Create project" });
  await projectDialog.getByLabel("Name", { exact: true }).fill(projectName);
  await projectDialog
    .getByLabel("Description", { exact: true })
    .fill("Synthetic project created by the browser acceptance flow.");
  await projectDialog
    .getByRole("button", { name: "Create", exact: true })
    .click();
  await expect(
    page.locator("a.project-card").filter({ hasText: projectName }),
  ).toBeVisible();
  assertNoBrowserErrors();
});
