import { expect, test } from "@playwright/test";

import { failOnBrowserErrors, loginAsDemo } from "./helpers";

test("production app registers its shell worker without caching API responses", async ({
  page,
}) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginAsDemo(page);
  const registration = await page.evaluate(async () => {
    const ready = await navigator.serviceWorker.ready;
    return { active: Boolean(ready.active), scope: ready.scope };
  });
  expect(registration.active).toBe(true);
  expect(registration.scope).toMatch(/\/$/);

  const apiWasCached = await page.evaluate(async () => {
    const cacheNames = await caches.keys();
    const hits = await Promise.all(
      cacheNames.map(async (name) =>
        Boolean(await (await caches.open(name)).match("/api/v1/me")),
      ),
    );
    return hits.some(Boolean);
  });
  expect(apiWasCached).toBe(false);

  await page.context().setOffline(true);
  await page.evaluate(() => window.dispatchEvent(new Event("offline")));
  await expect(page.getByRole("status")).toContainText(/offline/i);
  assertNoBrowserErrors();
});
