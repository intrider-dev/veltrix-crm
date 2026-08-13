import { test as setup } from "@playwright/test";

import {
  authStatePath,
  demoWorkspaceName,
  failOnBrowserErrors,
  loginWithCredentials,
  persistWorkspaceFixture,
  selectWorkspaceByName,
} from "./helpers";

setup("authenticate the local demo account once", async ({ page }) => {
  const assertNoBrowserErrors = failOnBrowserErrors(page);
  await loginWithCredentials(page);
  await selectWorkspaceByName(page, demoWorkspaceName);
  await persistWorkspaceFixture(page, demoWorkspaceName);
  await page
    .context()
    .storageState({ path: authStatePath, indexedDB: true });
  assertNoBrowserErrors();
});
