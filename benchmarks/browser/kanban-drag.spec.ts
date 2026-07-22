import { expect, test } from "@playwright/test";

import {
  failOnBrowserErrors,
  loginAsDemo,
  scenarioSuffix,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

test("desktop Kanban persists an actual pointer drag between stages", async ({
  page,
}, testInfo) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/deals");
  await waitForAppReady(page);

  const stages = page.locator("article.stage");
  await expect
    .poll(() => stages.count(), { timeout: 15_000 })
    .toBeGreaterThanOrEqual(2);
  const sourceStage = stages.first();
  const targetStage = stages.nth(1);
  const suffix = scenarioSuffix(testInfo.project.name);
  const dealName = `Dragged ${suffix}`;
  await page.getByRole("button", { name: "Add deal", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New deal" });
  await dialog.getByLabel("Name", { exact: true }).fill(dealName);
  await dialog.getByLabel("Amount", { exact: true }).fill("2400");
  await dialog.getByLabel("Currency", { exact: true }).fill("USD");
  await dialog.getByRole("button", { name: "Create", exact: true }).click();

  const card = sourceStage
    .locator("article.deal-card")
    .filter({ hasText: dealName });
  await expect(card).toBeVisible();
  const href = await card
    .getByRole("link", { name: dealName, exact: true })
    .getAttribute("href");
  expect(href).toMatch(/^\/deals\/[0-9a-f-]+$/);
  const dealId = href?.split("/").at(-1);
  expect(dealId).toMatch(/^[0-9a-f-]+$/);
  const handle = card.locator(".drag-handle");
  const dropZone = targetStage.locator(".drop-zone");
  await handle.scrollIntoViewIfNeeded();
  const sourceBox = await handle.boundingBox();
  const targetBox = await dropZone.boundingBox();
  expect(sourceBox, "drag handle bounding box").not.toBeNull();
  expect(targetBox, "target stage bounding box").not.toBeNull();

  // Columns can be several viewports tall. Dropping at `targetBox.y + 30`
  // would point above the viewport after the newly-created source card is
  // scrolled into view. Use the visible part of the adjacent drop zone while
  // still exercising actual pointer events through CDK drag-and-drop.
  const viewport = page.viewportSize();
  expect(viewport, "desktop viewport").not.toBeNull();
  const sourceX = sourceBox!.x + sourceBox!.width / 2;
  const sourceY = sourceBox!.y + sourceBox!.height / 2;
  const targetX = targetBox!.x + targetBox!.width / 2;
  const targetTop = Math.max(targetBox!.y + 8, 8);
  const targetBottom = Math.min(
    targetBox!.y + targetBox!.height - 8,
    viewport!.height - 8,
  );
  expect(targetBottom, "visible target drop-zone bottom").toBeGreaterThan(targetTop);
  const targetY = Math.min(Math.max(sourceY, targetTop), targetBottom);

  const moveResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().includes(`/deals/${dealId}/stage`),
  );
  await page.mouse.move(sourceX, sourceY);
  await page.mouse.down();
  await page.mouse.move(sourceX + 20, sourceY + 20, { steps: 5 });
  await page.mouse.move(targetX, targetY, { steps: 20 });
  await page.mouse.up();
  expect((await moveResponse).status()).toBe(200);
  await expect(
    targetStage.locator("article.deal-card").filter({ hasText: dealName }),
  ).toBeVisible();
  assertNoBrowserErrors();
});
