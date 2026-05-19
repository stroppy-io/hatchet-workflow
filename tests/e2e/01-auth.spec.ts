import { test, expect } from "@playwright/test";
import { login, getTenantId } from "./helpers";

// Functional auth flows: behaviour that breaks the product if it regresses.
test.describe("Authentication", () => {
  test("wrong credentials reject — user stays unauthenticated", async ({ page }) => {
    await page.goto("/login");
    await page.fill('input[name="username"], input[type="text"]', "admin");
    await page.fill('input[type="password"]', "definitely-wrong");
    await page.click('button[type="submit"]');
    // After bad auth: still on /login (or error surfaced), and protected area inaccessible.
    await page.waitForTimeout(1500);
    await page.goto("/t/any/runs");
    await page.waitForURL(/\/login/, { timeout: 5_000 });
  });

  test("correct credentials grant access to tenant scope", async ({ page }) => {
    await login(page);
    // Must land somewhere authenticated: tenant runs, select-tenant, or admin.
    expect(page.url().includes("/login")).toBeFalsy();
    expect(
      page.url().match(/\/t\/[^/]+/) ||
      page.url().includes("/select-tenant") ||
      page.url().includes("/admin/")
    ).toBeTruthy();
  });

  test("reload preserves authenticated session and tenant scope", async ({ page }) => {
    await login(page);
    if (!page.url().match(/\/t\/[^/]+/)) test.skip(true, "Not in tenant scope (admin / multi)");
    const tidBefore = getTenantId(page);
    await page.reload();
    await page.waitForTimeout(1500);
    expect(page.url().includes("/login")).toBeFalsy();
    expect(getTenantId(page)).toBe(tidBefore);
  });

  test("logout revokes access — protected pages redirect to login", async ({ page }) => {
    await login(page);
    // Find logout control in sidebar.
    const logout = page.locator('[title="Sign out"], button:has-text("Sign out"), [aria-label="logout"]').first();
    if (!(await logout.isVisible({ timeout: 2_000 }).catch(() => false))) {
      test.skip(true, "Logout control not exposed");
      return;
    }
    await logout.click();
    await page.waitForTimeout(1500);
    // Hitting any protected path now bounces to /login.
    await page.goto("/t/anything/runs");
    await page.waitForURL(/\/login/, { timeout: 5_000 });
  });

  test("deep-link to protected URL while logged out redirects with ?redirect=", async ({ page }) => {
    await page.goto("/t/abc/runs/some-id");
    await page.waitForURL(/\/login/, { timeout: 5_000 });
    expect(page.url()).toContain("redirect=");
  });
});
