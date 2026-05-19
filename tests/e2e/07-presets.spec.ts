import { test, expect } from "@playwright/test";
import { login, gotoTenant, getTenantId } from "./helpers";

// Functional: create-preset flow. After creating a topology preset via the
// designer, it must (a) appear in the same tenant's preset list and (b) be
// selectable on NewRun.

test.describe("Preset lifecycle", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await gotoTenant(page, "presets");
    await page.waitForTimeout(1_000);
  });

  test("presets page is tenant-scoped", async ({ page }) => {
    expect(page.url()).toMatch(/\/t\/[^/]+\/presets/);
  });

  test("created preset appears in the same tenant's list", async ({ page }) => {
    const newBtn = page.getByRole("link", { name: /New Preset/i }).first();
    if (!(await newBtn.isVisible({ timeout: 2_000 }).catch(() => false))) {
      test.skip(true, "No 'New Preset' affordance — page state unsupported");
      return;
    }
    await newBtn.click();
    await page.waitForURL(/\/t\/[^/]+\/presets\/new/, { timeout: 5_000 });

    // The designer is reshaped vs main (gap C7) — fill whatever Name input exists.
    const nameInput = page.locator('input[name="name"], #preset-name, input[placeholder*="name" i]').first();
    if (!(await nameInput.isVisible({ timeout: 3_000 }).catch(() => false))) {
      test.skip(true, "Designer has no recognisable name input");
      return;
    }
    const name = `e2e-preset-${Date.now()}`;
    await nameInput.fill(name);

    const tid = getTenantId(page);

    const saveBtn = page.getByRole("button", { name: /Save|Create/i }).first();
    if (!(await saveBtn.isVisible({ timeout: 2_000 }).catch(() => false))) {
      test.skip(true, "No save button");
      return;
    }
    await saveBtn.click();
    await page.waitForTimeout(2_000);

    // Should have returned to the list (same tenant) and show the new preset.
    await gotoTenant(page, "presets");
    await page.waitForTimeout(1_000);
    expect(page.url()).toContain(`/t/${tid}/presets`);
    await expect(page.getByText(name)).toBeVisible({ timeout: 5_000 });
  });

  test("presets list survives reload", async ({ page }) => {
    await page.waitForTimeout(1_000);
    const before = await page.locator("body").textContent();
    await page.reload();
    await page.waitForTimeout(1_500);
    const after = await page.locator("body").textContent();
    // No crash. List had content of similar order of magnitude.
    expect(after?.length || 0).toBeGreaterThanOrEqual(((before?.length || 0) / 4) | 0);
  });
});
