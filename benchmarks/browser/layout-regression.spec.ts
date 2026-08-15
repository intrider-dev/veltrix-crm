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
  const boxes = await container
    .locator(":scope > *:visible")
    .evaluateAll((elements) =>
      elements.map((element) => {
        const box = element.getBoundingClientRect();
        return { x: box.x, y: box.y, width: box.width, height: box.height };
      }),
    );
  for (const box of boxes) {
    expect.soft(box.x).toBeGreaterThanOrEqual((containerBox?.x ?? 0) - 1);
    expect
      .soft(box.x + box.width)
      .toBeLessThanOrEqual(
        (containerBox?.x ?? 0) + (containerBox?.width ?? 0) + 1,
      );
    expect.soft(box.y).toBeGreaterThanOrEqual((containerBox?.y ?? 0) - 1);
    expect
      .soft(box.y + box.height)
      .toBeLessThanOrEqual(
        (containerBox?.y ?? 0) + (containerBox?.height ?? 0) + 1,
      );
  }
  for (let first = 0; first < boxes.length; first += 1) {
    for (let second = first + 1; second < boxes.length; second += 1) {
      const horizontal =
        Math.min(
          boxes[first]!.x + boxes[first]!.width,
          boxes[second]!.x + boxes[second]!.width,
        ) - Math.max(boxes[first]!.x, boxes[second]!.x);
      const vertical =
        Math.min(
          boxes[first]!.y + boxes[first]!.height,
          boxes[second]!.y + boxes[second]!.height,
        ) - Math.max(boxes[first]!.y, boxes[second]!.y);
      expect.soft(Math.max(0, horizontal) * Math.max(0, vertical)).toBe(0);
    }
  }
}

test("forgot-password action follows the password field", async ({
  context,
  page,
}) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await context.clearCookies();
  await page.goto("/login");

  const password = page.locator('input[autocomplete="current-password"]');
  const forgot = page.getByRole("link", {
    name: "Forgot password?",
    exact: true,
  });
  await expect(password).toBeVisible();
  await expect(forgot).toBeVisible();
  const passwordBox = await password.boundingBox();
  const forgotBox = await forgot.boundingBox();
  expect(passwordBox).not.toBeNull();
  expect(forgotBox).not.toBeNull();
  expect(forgotBox?.y ?? 0).toBeGreaterThanOrEqual(
    (passwordBox?.y ?? 0) + (passwordBox?.height ?? 0),
  );
  assertNoBrowserErrors();
});

for (const route of ["/contacts", "/companies"] as const) {
  test(`${route} toolbar keeps controls aligned and inset`, async ({
    page,
  }) => {
    const assertNoBrowserErrors = failOnBrowserErrors(page);
    await loginAsDemo(page);
    await setAppLanguage(page, "en");
    await page.goto(route);
    await waitForAppReady(page);

    const toolbar = page.locator(".filter-toolbar");
    await expect(toolbar).toBeVisible();
    await expectChildrenContainedAndSeparated(toolbar);

    const search =
      route === "/companies"
        ? page.locator(".header-search input")
        : toolbar.locator('input[type="search"]');
    await expect(search).toBeVisible();
    await expect(toolbar.locator(".filter-search button")).toHaveCount(0);

    const select = toolbar.locator("mat-select").first();
    const selectBox = await select.boundingBox();
    const arrowBox = await select.locator("svg").boundingBox();
    expect(selectBox).not.toBeNull();
    expect(arrowBox).not.toBeNull();
    expect(
      (selectBox?.x ?? 0) +
        (selectBox?.width ?? 0) -
        ((arrowBox?.x ?? 0) + (arrowBox?.width ?? 0)),
    ).toBeGreaterThanOrEqual(8);

    const segmented =
      route === "/contacts"
        ? page.locator(".contacts-content .segmented-control")
        : toolbar.locator(".segmented-control");
    const radius = await segmented.evaluate((element) =>
      Number.parseFloat(getComputedStyle(element).borderRadius),
    );
    expect(radius).toBeGreaterThanOrEqual(6);

    if (route === "/companies") {
      const add = page.getByRole("button", {
        name: "Add company",
        exact: true,
      });
      const buttonBox = await add.boundingBox();
      const iconBox = await add.locator("app-icon").boundingBox();
      expect(buttonBox).not.toBeNull();
      expect(iconBox).not.toBeNull();
      expect(
        Math.abs(
          (buttonBox?.y ?? 0) +
            (buttonBox?.height ?? 0) / 2 -
            ((iconBox?.y ?? 0) + (iconBox?.height ?? 0) / 2),
        ),
      ).toBeLessThanOrEqual(3);
    }
    assertNoBrowserErrors();
  });
}

test("chat creation panel stays contained without overlapping controls", async ({
  page,
}, testInfo) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/dashboard");
  await waitForAppReady(page);

  await page.getByRole("button", { name: "Open team chat" }).click();
  const dock = page.getByRole("dialog", { name: "Team chat" });
  await expect(dock).toBeVisible();
  await expect(dock).toHaveAttribute(
    "aria-modal",
    testInfo.project.name === "mobile-390" ? "true" : "false",
  );
  await dock.getByRole("button", { name: "New conversation" }).click();

  const panel = dock.locator(".new-conversation");
  await expect(panel).toBeVisible();
  await expectChildrenContainedAndSeparated(panel);
  const select = panel.getByRole("combobox", { name: "Choose a colleague" });
  await expect(select).toBeVisible();
  await expect(select).toBeFocused();
  const selectBox = await panel.locator("mat-form-field").boundingBox();
  expect(selectBox?.height ?? 0).toBeGreaterThanOrEqual(40);
  const actions = panel.getByRole("contentinfo");
  await expect(actions.getByRole("button", { name: "Cancel" })).toBeVisible();
  await expect(actions.getByRole("button", { name: "Create" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Open team chat" }),
  ).toHaveCount(0);
  await page.keyboard.press("Escape");
  await expect(panel).toHaveCount(0);
  await expect(
    dock.getByRole("button", { name: "New conversation" }),
  ).toBeFocused();
  assertNoBrowserErrors();
});

test("mobile navigation contains focus and restores its trigger", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-390", "mobile-only behavior");
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");
  await page.goto("/dashboard");
  await waitForAppReady(page);

  const trigger = page.getByRole("button", { name: "Open navigation" });
  const navigation = page.locator("aside.sidebar");
  await expect(navigation).toHaveAttribute("aria-hidden", "true");
  await expect(navigation).toHaveAttribute("inert", "");
  await trigger.click();
  await expect(trigger).toHaveAttribute("aria-expanded", "true");
  await expect(navigation).not.toHaveAttribute("aria-hidden", "true");
  await expect(navigation).not.toHaveAttribute("inert", "");
  await expect(
    navigation.getByRole("button", { name: "Close navigation" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
  await expect(navigation).toHaveAttribute("aria-hidden", "true");
  await expect(navigation).toHaveAttribute("inert", "");
  await expect(trigger).toBeFocused();
  assertNoBrowserErrors();
});

test("mobile creation drawers trap focus and close with Escape", async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== "mobile-390", "mobile-only behavior");
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  await setAppLanguage(page, "en");

  await page.goto("/projects");
  await waitForAppReady(page);
  const projectTrigger = page.getByRole("button", { name: "New project" });
  await projectTrigger.click();
  const projectDialog = page.getByRole("dialog", { name: "Create project" });
  await expect(projectDialog.getByLabel("Name", { exact: true })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(projectDialog).toHaveCount(0);
  await expect(projectTrigger).toBeFocused();

  await page.goto("/deals");
  await waitForAppReady(page);
  const dealTrigger = page.getByRole("button", { name: "Add deal" });
  await dealTrigger.click();
  const dealDialog = page.getByRole("dialog", { name: "New deal" });
  await expect(dealDialog.getByLabel("Name", { exact: true })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dealDialog).toHaveCount(0);
  await expect(dealTrigger).toBeFocused();
  assertNoBrowserErrors();
});
