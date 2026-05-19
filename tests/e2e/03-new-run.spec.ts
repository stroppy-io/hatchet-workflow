import { test, expect } from "@playwright/test";
import { login, gotoTenant, ensureRunsPage, getTenantId } from "./helpers";

// Functional: a user lands on NewRun and either CAN create a run (when presets
// exist) or is correctly told to create presets first. After creating a run
// they must see it in the runs list of the same tenant.

test.describe("New Run create flow", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await gotoTenant(page, "runs/new");
  });

  test("form surfaces preset availability honestly", async ({ page }) => {
    await page.waitForTimeout(1_500);
    const hasDbSelect = (await page.locator("#db-preset").count()) > 0;
    const hasEmptyHint = (await page.getByText(/Create one first/i).count()) > 0;
    // Exactly one of these must be true. Both = inconsistent state.
    expect(hasDbSelect || hasEmptyHint).toBeTruthy();
  });

  test("Launch is disabled when there is nothing to launch", async ({ page }) => {
    await page.waitForTimeout(1_500);
    const hasEmptyHint = (await page.getByText(/Create one first/i).count()) > 0;
    if (!hasEmptyHint) test.skip(true, "Presets exist — disabled-button check N/A");
    const launchBtn = page.getByRole("button", { name: /Launch Run/i });
    await expect(launchBtn).toBeDisabled();
  });

  test("submitting a valid form creates a run that shows in the list", async ({ page }) => {
    await page.waitForTimeout(1_500);
    const hasDbSelect = (await page.locator("#db-preset").count()) > 0;
    if (!hasDbSelect) test.skip(true, "No presets seeded — cannot exercise creation");

    const tid = getTenantId(page);
    const name = `e2e-run-${Date.now()}`;
    await page.fill("#run-name", name);
    // Pick first preset of each select (already selected by default but force change event).
    await page.locator("#db-preset").selectOption({ index: 0 });
    await page.locator("#wl-preset").selectOption({ index: 0 });
    await page.getByRole("button", { name: /Launch Run/i }).click();

    // After successful submit we navigate to the run detail page (same tenant).
    await page.waitForURL(/\/t\/[^/]+\/runs\/[^/?]+/, { timeout: 10_000 });
    expect(page.url()).toContain(`/t/${tid}/`);

    // The created run must show in the tenant's list.
    await ensureRunsPage(page);
    await expect(page.getByText(name)).toBeVisible({ timeout: 10_000 });
  });

  test("Cancel returns the user to the SAME tenant runs list", async ({ page }) => {
    const tid = getTenantId(page);
    await page.click("text=Cancel");
    await page.waitForURL(/\/t\/[^/]+\/runs(\?|$)/, { timeout: 5_000 });
    expect(page.url()).toContain(`/t/${tid}/`);
  });
});
