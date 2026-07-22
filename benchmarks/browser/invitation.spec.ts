import { expect, test } from "@playwright/test";

import {
  currentWorkspace,
  failOnBrowserErrors,
  loginAsDemo,
  loginWithCredentials,
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
  const acceptButton = page.getByRole("button", {
    name: "Accept invitation",
    exact: true,
  });
  await expect(acceptButton).toBeVisible();
  await page.evaluate(() =>
    history.replaceState({}, "", "/invitations/accept"),
  );
  await acceptButton.click();
  await expect(page.getByRole("status")).toContainText("Invitation accepted");
  await page
    .getByRole("button", { name: "Open workspace", exact: true })
    .click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.locator("mat-select.workspace-select")).toContainText(
    invitedWorkspace.name,
  );

  // The invited sales role cannot administer the workspace member directory.
  // Return to the owner account to exercise a permitted direct-chat creation
  // against the user that has just joined.
  await context.clearCookies();
  await loginWithCredentials(page);
  await expect(page.locator("mat-select.workspace-select")).toContainText(
    invitedWorkspace.name,
  );

  await page
    .getByRole("button", { name: "Open team chat", exact: true })
    .click();
  const chat = page.getByRole("dialog", { name: "Team chat" });
  await expect(chat).toBeVisible();
  await chat
    .getByRole("button", { name: "New conversation", exact: true })
    .click();
  const memberSelect = chat.locator(".new-conversation select");
  await expect
    .poll(() => memberSelect.locator("option").count(), { timeout: 15_000 })
    .toBeGreaterThan(1);
  await memberSelect.selectOption({ label: `Invited ${suffix}` });
  await chat.getByRole("button", { name: "Create", exact: true }).click();

  const message = `Hello from ${suffix}`;
  await chat.getByPlaceholder("Message").fill(message);
  await chat.getByRole("button", { name: "Send", exact: true }).click();
  await expect(chat.locator(".message-list")).toContainText(message);
  await expect(chat.getByTitle("Calls are not configured")).toHaveCount(2);
  await expect(
    chat.getByTitle("Calls are not configured").first(),
  ).toBeDisabled();

  // Verify the persisted conversation from the recipient's own authorized
  // session, then send a reply and verify it from the owner session. This
  // proves bidirectional access without sharing a browser session or token.
  await context.clearCookies();
  await page.goto("/login");
  await page.locator('input[autocomplete="username"]').fill(email);
  await page.locator('input[autocomplete="current-password"]').fill(password);
  await page.locator('button[type="submit"]').click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await page
    .getByRole("button", { name: "Open team chat", exact: true })
    .click();
  const recipientChat = page.getByRole("dialog", { name: "Team chat" });
  await expect(recipientChat).toBeVisible();
  const recipientConversation = recipientChat.locator(
    ".conversation-list > button",
  );
  await expect(recipientConversation).toHaveCount(1);
  await recipientConversation.click();
  await expect(recipientChat.locator(".message-list")).toContainText(message);
  const reply = `Reply from ${suffix}`;
  await recipientChat.getByPlaceholder("Message").fill(reply);
  await recipientChat.getByRole("button", { name: "Send", exact: true }).click();
  await expect(recipientChat.locator(".message-list")).toContainText(reply);

  await context.clearCookies();
  await loginWithCredentials(page);
  await page
    .getByRole("button", { name: "Open team chat", exact: true })
    .click();
  const ownerChat = page.getByRole("dialog", { name: "Team chat" });
  const ownerConversation = ownerChat
    .locator(".conversation-list > button")
    .filter({ hasText: `Invited ${suffix}` });
  await expect(ownerConversation).toHaveCount(1);
  await ownerConversation.click();
  await expect(ownerChat.locator(".message-list")).toContainText(reply);
  assertNoBrowserErrors();
});
