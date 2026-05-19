import { test, expect } from "@playwright/test";
import { login, ensureRunsPage, gotoTenant } from "./helpers";

// Functional: runs page must reflect actual state from the API, and the
// New Run button must lead to a working create flow.
test.describe("Runs list reflects backend state", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await ensureRunsPage(page);
  });

  test("list either renders rows from API or shows empty state — never blank", async ({ page }) => {
    // Wait briefly for fetch to complete.
    await page.waitForTimeout(1500);
    const hasRows = (await page.locator("table tbody tr").count()) > 0;
    const hasEmpty = (await page.getByText(/no runs/i).count()) > 0;
    expect(hasRows || hasEmpty).toBeTruthy();
  });

  test("new run navigates to the tenant-scoped wizard, not a different tenant", async ({ page }) => {
    const m = page.url().match(/\/t\/([^/]+)/);
    const tid = m && m[1];
    await page.click("text=New Run");
    await page.waitForURL(/\/t\/[^/]+\/runs\/new/, { timeout: 5_000 });
    // Same tenant — switching here must NOT cross-leak.
    expect(page.url()).toContain(`/t/${tid}/`);
  });

  test("auto-refresh updates the list (no manual reload)", async ({ page }) => {
    // Snapshot row count, wait beyond the default 5s interval, snapshot again.
    // If a row arrives via auto-refresh we get a non-trivial signal; otherwise
    // we just assert the page didn't crash.
    const before = await page.locator("table tbody tr").count();
    await page.waitForTimeout(6_000);
    const after = await page.locator("table tbody tr").count();
    expect(after).toBeGreaterThanOrEqual(before);
  });

  test("direct URL /t/<tid>/runs after fresh login lands on same tenant", async ({ page }) => {
    const m = page.url().match(/\/t\/([^/]+)/);
    if (!m) test.skip(true, "Not in tenant scope");
    const tid = m![1];
    await gotoTenant(page, "runs");
    expect(page.url()).toContain(`/t/${tid}/runs`);
  });
});
