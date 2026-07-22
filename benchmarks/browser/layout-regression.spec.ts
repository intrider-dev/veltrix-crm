import type { Locator } from "@playwright/test";
import { expect, test } from "@playwright/test";

import {
  failOnBrowserErrors,
  loginAsDemo,
  setAppLanguage,
  waitForAppReady,
} from "./helpers";

async function expectChildrenContainedAndSeparated(container: Locator) {
  const containerBox = await container.boundingBox();
  expect(containerBox).not.toBeNull();
  const boxes = await container.locator(":scope > *:visible").evaluateAll((elements) =>
    elements.map((element) => {
      const box = element.getBoundingClientRect();
      return { x: box.x, y: box.y, width: box.width, height: box.height };
    }),
  );
  for (const box of boxes) {
    expect.soft(box.x).toBeGreaterThanOrEqual((containerBox?.x ?? 0) - 1);
    expect
      .soft(box.x + box.width)
      .toBeLessThanOrEqual((containerBox?.x ?? 0) + (containerBox?.width ?? 0) + 1);
    expect.soft(box.y).toBeGreaterThanOrEqual((containerBox?.y ?? 0) - 1);
    expect
      .soft(box.y + box.height)
      .toBeLessThanOrEqual((containerBox?.y ?? 0) + (containerBox?.height ?? 0) + 1);
  }
  for (let first = 0; first < boxes.length; first += 1) {
    for (let second = first + 1; second < boxes.length; second += 1) {
      const horizontal = Math.min(
        boxes[first]!.x + boxes[first]!.width,
        boxes[second]!.x + boxes[second]!.width,
      ) - Math.max(boxes[first]!.x, boxes[second]!.x);
      const vertical = Math.min(
        boxes[first]!.y + boxes[first]!.height,
        boxes[second]!.y + boxes[second]!.height,
      ) - Math.max(boxes[first]!.y, boxes[second]!.y);
      expect.soft(Math.max(0, horizontal) * Math.max(0, vertical)).toBe(0);
    }
  }
}

for (const route of ["/contacts", "/companies"] as const) {
  test(`${route} toolbar keeps controls aligned and inset`, async ({ page }) => {
    const assertNoBrowserErrors = failOnBrowserErrors(page);
    await loginAsDemo(page);
    await setAppLanguage(page, "en");
    await page.goto(route);
    await waitForAppReady(page);

    const toolbar = page.locator(".filter-toolbar");
    await expect(toolbar).toBeVisible();
    await expectChildrenContainedAndSeparated(toolbar);

    const search = toolbar.locator('input[type="search"]');
    await expect(search).toBeVisible();
    await expect(toolbar.locator(".filter-search button")).toHaveCount(0);

    const selectStyle = await toolbar.locator("select").first().evaluate((element) => {
      const style = getComputedStyle(element);
      return {
        appearance: style.appearance,
        backgroundPosition: style.backgroundPosition,
        paddingRight: Number.parseFloat(style.paddingRight),
      };
    });
    expect(selectStyle.appearance).toBe("none");
    expect(selectStyle.backgroundPosition).toContain("calc(100% -");
    expect(selectStyle.paddingRight).toBeGreaterThanOrEqual(32);

    const segmented = toolbar.locator(".segmented-control");
    const radius = await segmented.evaluate((element) =>
      Number.parseFloat(getComputedStyle(element).borderRadius),
    );
    expect(radius).toBeGreaterThanOrEqual(6);

    if (route === "/companies") {
      const add = page.getByRole("button", { name: "Add", exact: true });
      const buttonBox = await add.boundingBox();
      const iconBox = await add.locator("app-icon").boundingBox();
      expect(buttonBox).not.toBeNull();
      expect(iconBox).not.toBeNull();
      expect(
        Math.abs(
          (buttonBox?.y ?? 0) + (buttonBox?.height ?? 0) / 2 -
            ((iconBox?.y ?? 0) + (iconBox?.height ?? 0) / 2),
        ),
      ).toBeLessThanOrEqual(3);
    }
    assertNoBrowserErrors();
  });
}

test("chat creation panel stays contained without overlapping controls", async ({ page }) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/dashboard");
  await waitForAppReady(page);

  await page.getByRole("button", { name: "Open team chat" }).click();
  const dock = page.getByRole("dialog", { name: "Team chat" });
  await expect(dock).toBeVisible();
  await dock.getByRole("button", { name: "New conversation" }).click();

  const panel = dock.locator(".new-conversation");
  await expect(panel).toBeVisible();
  await expectChildrenContainedAndSeparated(panel);
  const select = panel.locator("select");
  const selectBox = await select.boundingBox();
  expect(selectBox?.height ?? 0).toBeGreaterThanOrEqual(40);
  await expect(panel.getByRole("button", { name: "Cancel" })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Create" })).toBeVisible();
  assertNoBrowserErrors();
});
