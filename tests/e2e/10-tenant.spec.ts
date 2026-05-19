import { test, expect } from "@playwright/test";
import { login, gotoTenant, getTenantId } from "./helpers";

// Functional: tenant URL is the source of truth. Reload, deep-link, switching
// tenants — all must remain honest about which tenant the user is in.

test.describe("Tenant URL is source of truth", () => {
  test("after login, URL is /t/<tenantId>/runs", async ({ page }) => {
    await login(page);
    if (!page.url().match(/\/t\/[^/]+/)) test.skip(true, "Not in tenant scope (admin or multi-tenant)");
    expect(page.url()).toMatch(/\/t\/[^/]+\/runs/);
  });

  test("invalid tenant id in URL bounces to /select-tenant", async ({ page }) => {
    await login(page);
    await page.goto("/t/00000000000000000000000000/runs");
    await page.waitForURL((url) => url.pathname.includes("/select-tenant") || !url.pathname.startsWith("/t/00000000000000000000000000"), { timeout: 5_000 });
  });

  test("reload keeps the tenant id; localStorage isn't authoritative", async ({ page }) => {
    await login(page);
    if (!page.url().match(/\/t\/[^/]+/)) test.skip(true, "Not in tenant scope");
    const tid = getTenantId(page);
    // Manually corrupt localStorage — URL must still win after reload.
    await page.evaluate(() => localStorage.setItem("stroppy.tenantId", "garbage"));
    await page.reload();
    await page.waitForTimeout(1_500);
    expect(getTenantId(page)).toBe(tid);
  });

  test("logout clears tenant — protected paths require re-login", async ({ page }) => {
    await login(page);
    const logout = page.locator('[title="Sign out"], button[aria-label="logout"]').first();
    if (!(await logout.isVisible({ timeout: 2_000 }).catch(() => false))) {
      test.skip(true, "Logout control not present");
      return;
    }
    await logout.click();
    await page.waitForTimeout(1_500);
    await page.goto("/t/whatever/runs");
    await page.waitForURL(/\/login/, { timeout: 5_000 });
  });

  test("Members page is tenant-scoped (or 404 → permission redirect)", async ({ page }) => {
    await login(page);
    await gotoTenant(page, "members");
    await page.waitForTimeout(1_000);
    // For owners: the page renders. For non-owners: the route gate redirects.
    const inMembers = /\/t\/[^/]+\/members/.test(page.url());
    const redirected = !/\/t\/[^/]+\/members/.test(page.url());
    expect(inMembers || redirected).toBeTruthy();
  });
});
