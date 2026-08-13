import { expect, type Locator, type Page } from "@playwright/test";

export const authStatePath =
  process.env["E2E_AUTH_STATE"] ?? "benchmarks/results/e2e-auth.json";
export const demoEmail = process.env["E2E_EMAIL"] ?? "admin@demo.local";
export const demoPassword = process.env["E2E_PASSWORD"] ?? "Demo123!";
export const demoWorkspaceName =
  process.env["E2E_WORKSPACE_NAME"] ?? "Small synthetic dataset";

export interface SessionView {
  readonly user: { readonly id: string; readonly email: string };
  readonly workspaces: ReadonlyArray<{
    readonly id: string;
    readonly name: string;
  }>;
}

export async function loginWithCredentials(page: Page): Promise<void> {
  await page.goto("/login");
  await page.locator('input[autocomplete="username"]').fill(demoEmail);
  await page
    .locator('input[autocomplete="current-password"]')
    .fill(demoPassword);
  await page.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.locator("main#main-content")).toBeVisible();
}

/**
 * Opens an authenticated route while retaining a credential-login fallback for
 * local runs that intentionally omit the setup project.
 */
export async function loginAsDemo(page: Page): Promise<void> {
  await page.goto("/dashboard");
  if (/\/login(?:\?|$)/.test(page.url())) await loginWithCredentials(page);
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.locator("main#main-content")).toBeVisible();
}

export async function setAppLanguage(
  page: Page,
  locale: "en" | "ru",
): Promise<void> {
  await page.goto("/settings");
  await waitForAppReady(page);
  const expectedHeading = locale === "en" ? "Settings" : "Настройки";
  if (
    await page
      .getByRole("heading", { level: 1, name: expectedHeading })
      .isVisible()
  )
    return;
  const currentLocale =
    ((await page.locator("html").getAttribute("lang")) ?? "en").split("-")[0] ??
    "en";
  const displayName =
    new Intl.DisplayNames([currentLocale], { type: "language" }).of(locale) ??
    locale;
  const optionName =
    displayName.charAt(0).toLocaleUpperCase(currentLocale) +
    displayName.slice(1);
  await page
    .getByRole("button", {
      name: optionName,
      exact: true,
    })
    .click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    expectedHeading,
  );
}

export async function waitForAppReady(page: Page): Promise<void> {
  await expect(page.locator("main#main-content")).toBeVisible();
  await expect(page.locator(".skeleton")).toHaveCount(0, { timeout: 20_000 });
}

export function firstGridDataRow(page: Page): Locator {
  return page.getByRole("grid").getByRole("row").nth(1);
}

export async function createContact(
  page: Page,
  name: { readonly first: string; readonly last: string },
  email: string,
): Promise<string> {
  await page.goto("/contacts");
  await waitForAppReady(page);
  await page.getByRole("button", { name: "Add contact", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New contact" });
  await dialog.getByLabel("First name").fill(name.first);
  await dialog.getByLabel("Last name").fill(name.last);
  await dialog.getByLabel("Email", { exact: true }).fill(email);
  await activateButtonWithKeyboard(
    dialog.getByRole("button", { name: "Create", exact: true }),
  );
  await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]+$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    `${name.first} ${name.last}`,
  );
  return page.url().split("/").at(-1) ?? "";
}

export async function createCompany(
  page: Page,
  input: {
    readonly name: string;
    readonly domain: string;
    readonly industry?: string;
  },
): Promise<string> {
  await page.goto("/companies");
  await waitForAppReady(page);
  await page.getByRole("button", { name: "Add company", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "New company" });
  await dialog.getByLabel("Name", { exact: true }).fill(input.name);
  await dialog.getByLabel("Domain", { exact: true }).fill(input.domain);
  if (input.industry)
    await dialog.getByLabel("Industry", { exact: true }).fill(input.industry);
  await activateButtonWithKeyboard(
    dialog.getByRole("button", { name: "Create", exact: true }),
  );
  await expect(page).toHaveURL(/\/companies\/[0-9a-f-]+$/);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(input.name);
  return page.url().split("/").at(-1) ?? "";
}

export async function openGlobalSearch(
  page: Page,
  query: string,
): Promise<Locator> {
  await page
    .getByRole("button", { name: "Global search", exact: true })
    .click();
  const palette = page.getByRole("dialog", { name: "Command palette" });
  await palette.getByRole("searchbox", { name: "Global search" }).fill(query);
  return palette;
}

export async function currentSession(page: Page): Promise<SessionView> {
  return page.evaluate(async () => {
    const response = await fetch("/api/v1/me");
    if (!response.ok)
      throw new Error(`GET /api/v1/me failed with ${response.status}`);
    return (await response.json()) as SessionView;
  });
}

export async function currentWorkspace(
  page: Page,
): Promise<{ readonly id: string; readonly name: string }> {
  const name = (
    await page.locator("mat-select.workspace-select").innerText()
  ).trim();
  const session = await currentSession(page);
  const workspace = session.workspaces.find((item) => item.name === name);
  if (!workspace)
    throw new Error(
      `Active workspace ${JSON.stringify(name)} is absent from the session`,
    );
  return workspace;
}

/** Selects a workspace through the product UI and waits for route/data reset. */
export async function selectWorkspaceByName(
  page: Page,
  workspaceName: string,
): Promise<void> {
  const selector = page.locator("mat-select.workspace-select");
  await expect(selector).toBeVisible();
  if ((await selector.innerText()).trim() !== workspaceName) {
    await selector.click();
    await page
      .getByRole("option", { name: workspaceName, exact: true })
      .click();
    await expect(page).toHaveURL(/\/dashboard$/);
    await waitForAppReady(page);
  }
  await expect(selector).toContainText(workspaceName);
}

/**
 * Captures the demo workspace as an IndexedDB-backed browser preference.
 * Playwright contexts are isolated, so including this origin state prevents a
 * workspace created by one project from becoming the next project's implicit
 * first workspace while still exercising the application's own preference
 * format and restore path.
 */
export async function persistWorkspaceFixture(
  page: Page,
  workspaceName: string,
): Promise<void> {
  const session = await currentSession(page);
  const workspace = session.workspaces.find(
    (item) => item.name === workspaceName,
  );
  if (!workspace)
    throw new Error(
      `Workspace fixture ${JSON.stringify(workspaceName)} is absent`,
    );
  await page.evaluate(async (selected) => {
    const databaseName = (await indexedDB.databases())
      .map((database) => database.name)
      .find((name): name is string => Boolean(name?.endsWith("-app-state")));
    if (!databaseName)
      throw new Error("The application's IndexedDB state database is absent");
    const database = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open(databaseName, 2);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    await new Promise<void>((resolve, reject) => {
      const transaction = database.transaction("preferences", "readwrite");
      transaction.objectStore("preferences").put(
        {
          selectedId: selected.id,
          recent: [{ ...selected, accessedAt: Date.now() }],
        },
        "workspace",
      );
      transaction.oncomplete = () => resolve();
      transaction.onerror = () => reject(transaction.error);
      transaction.onabort = () => reject(transaction.error);
    });
    database.close();
  }, workspace);
}

export async function postAsCurrentUser<T>(
  page: Page,
  path: string,
  body: unknown,
): Promise<T> {
  return page.evaluate(
    async ({ requestPath, requestBody }) => {
      const csrf = document.cookie
        .split(";")
        .map((part) => part.trim())
        .find((part) => part.startsWith("XSRF-TOKEN="))
        ?.slice("XSRF-TOKEN=".length);
      if (!csrf) throw new Error("The readable CSRF cookie is missing");
      const response = await fetch(requestPath, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
          "X-CSRF-Token": decodeURIComponent(csrf),
        },
        body: JSON.stringify(requestBody),
      });
      if (!response.ok) {
        const problem = await response.text();
        throw new Error(
          `POST ${requestPath} failed with ${response.status}: ${problem}`,
        );
      }
      return (await response.json()) as T;
    },
    { requestPath: path, requestBody: body },
  );
}

export function scenarioSuffix(projectName: string): string {
  return `${projectName.replace(/[^a-z0-9]+/gi, "-").toLowerCase()}-${Date.now().toString(36)}`;
}

export function failOnBrowserErrors(page: Page): () => void {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(`pageerror: ${error.message}`));
  page.on("console", (message) => {
    if (message.type() === "error") errors.push(`console: ${message.text()}`);
  });
  return () => expect(errors, "browser console and page errors").toEqual([]);
}

async function activateButtonWithKeyboard(button: Locator): Promise<void> {
  await button.focus();
  await expect(button).toBeFocused();
  await button.press("Enter");
}
