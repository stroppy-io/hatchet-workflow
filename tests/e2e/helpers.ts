import { type Page, expect } from "@playwright/test";

/** Login as admin and wait for redirect into the tenant-scoped UI. */
export async function login(page: Page, user = "admin", pass = "admin") {
  // Wipe cookies + storage so the HttpOnly refresh-cookie set by a previous
  // test cannot silently re-authenticate this one (AuthContext does a silent
  // refresh on mount; with a leftover cookie /login redirects to
  // /admin/tenants before the form ever renders).
  await page.context().clearCookies();
  await page.goto("/login");
  await page.evaluate(() => {
    try { localStorage.clear(); sessionStorage.clear(); } catch {}
  });
  await page.reload();
  await page.fill('input[name="username"], input[type="text"]', user);
  await page.fill('input[type="password"]', pass);
  await page.click('button[type="submit"]');
  // Wait until React Router settles onto a real destination, not the
  // intermediate "/" that AppRoutes uses before calling defaultLanding.
  // Acceptable terminal paths: /t/<id>/..., /select-tenant, /admin/...
  await page.waitForURL(
    (url) =>
      /\/t\/[^/]+/.test(url.pathname) ||
      url.pathname.startsWith("/select-tenant") ||
      url.pathname.startsWith("/admin/"),
    { timeout: 10_000 }
  );
}

/**
 * getTenantId reads the active tenant from the current URL. Throws if the
 * page is not under /t/:tenantId/. Pages outside the tenant scope (admin,
 * select-tenant, share) have no tenant id by design.
 */
export function getTenantId(page: Page): string {
  const m = new URL(page.url()).pathname.match(/^\/t\/([^/]+)/);
  if (!m) throw new Error(`page not under /t/:tenantId/: ${page.url()}`);
  return m[1];
}

/** tUrl builds a tenant-scoped path using the current tenant id from the page. */
export function tUrl(page: Page, path: string): string {
  const tid = getTenantId(page);
  const clean = path.startsWith("/") ? path.slice(1) : path;
  return `/t/${tid}/${clean}`;
}

/**
 * ensureTenant guarantees the page is in tenant scope after login. When a root
 * admin lands on /admin/tenants with zero tenants, this creates one through
 * the AdminTenants UI ("e2e-default") and waits for the redirect to
 * /t/<newId>/runs. Idempotent — does nothing when already in tenant scope.
 */
export async function ensureTenant(page: Page) {
  if (page.url().match(/\/t\/[^/]+/)) return;
  // /admin/tenants — create one via the UI (Add Tenant button → fill → submit).
  if (!page.url().includes("/admin/tenants")) {
    await page.goto("/admin/tenants");
  }
  // Open the create dialog.
  await page.getByRole("button", { name: /add tenant|new tenant|create/i }).first().click();
  // Fill the name input inside the dialog.
  const nameInput = page.locator('input[type="text"], input:not([type])').first();
  await nameInput.fill("e2e-default");
  // Confirm.
  await page.getByRole("button", { name: /^create$|^add$|^save$/i }).first().click();
  // Backend substitutes admin as OWNER and AdminTenants navigates to /t/<id>/runs.
  await page.waitForURL(/\/t\/[^/]+\/runs/, { timeout: 10_000 });
}

/**
 * gotoTenant logs in (if not already) and navigates to a tenant-scoped path.
 * Use this in place of page.goto("/runs/new") etc. — handles the /t/<id>/
 * prefix automatically. Creates a default tenant for root admins if missing.
 */
export async function gotoTenant(page: Page, path: string) {
  // If we're not authenticated, log in first so we have a tenant id.
  if (!page.url().match(/\/t\/[^/]+/)) {
    if (!page.url().includes("/login") && page.url() !== "about:blank") {
      await login(page);
    } else if (page.url() === "about:blank") {
      await login(page);
    }
  }
  // Root admin with no tenants landed on /admin/tenants — create one.
  if (!page.url().match(/\/t\/[^/]+/)) {
    await ensureTenant(page);
  }
  if (!page.url().match(/\/t\/[^/]+/)) {
    throw new Error(`after login + ensureTenant, page not in tenant scope: ${page.url()}`);
  }
  await page.goto(tUrl(page, path));
}

/** Ensure we're on the runs list page (tenant-scoped). */
export async function ensureRunsPage(page: Page) {
  if (!page.url().match(/\/t\/[^/]+\/runs(\?|$|\/)/)) {
    await gotoTenant(page, "runs");
  }
  await page.waitForSelector("text=New Run", { timeout: 10_000 });
}

/** Wait for text to appear anywhere on the page. */
export async function waitForText(page: Page, text: string, timeout = 10_000) {
  await expect(page.locator(`text=${text}`).first()).toBeVisible({ timeout });
}

/** Get the value of an input by label text. */
export async function getInputValue(page: Page, label: string): Promise<string> {
  const input = page.locator(`label:has-text("${label}") ~ input, label:has-text("${label}") + * input`).first();
  return (await input.inputValue()) || "";
}
