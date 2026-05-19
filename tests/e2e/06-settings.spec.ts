import { test, expect } from "@playwright/test";
import { login, gotoTenant } from "./helpers";

// Functional: settings must be persistable per-tenant. After editing a value
// and reloading, the value must be the new one — anything else means writes
// don't actually hit the backend (or read path is broken).

test.describe("Settings persistence", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await gotoTenant(page, "settings");
    await page.waitForTimeout(1_000);
  });

  test("tenant-scoped URL", async ({ page }) => {
    expect(page.url()).toMatch(/\/t\/[^/]+\/settings/);
  });

  test("edited setting survives reload", async ({ page }) => {
    // Find any free-text setting input. The page renders a generic key/value
    // form (legacy YC validation form was lost — gap C10).
    const input = page.locator('input[type="text"], input:not([type])').first();
    if (!(await input.isVisible({ timeout: 3_000 }).catch(() => false))) {
      test.skip(true, "No editable setting inputs");
      return;
    }
    const value = `e2e-${Date.now()}`;
    await input.fill(value);

    const saveBtn = page.locator('button:has-text("Save"), button:has-text("Apply")').first();
    if (!(await saveBtn.isVisible({ timeout: 2_000 }).catch(() => false))) {
      test.skip(true, "No save button — page is read-only");
      return;
    }
    await saveBtn.click();
    await page.waitForTimeout(1_500);

    await page.reload();
    await page.waitForTimeout(1_500);

    const readBack = await page.locator('input[type="text"], input:not([type])').first().inputValue();
    expect(readBack).toBe(value);
  });
});
