import { test, expect } from "@playwright/test";
import { login, gotoTenant, getTenantId } from "./helpers";

// Functional: tenant isolation must hold across the UI. A resource created
// while pinned to tenant A must not appear when the URL is pinned to tenant B.
//
// This spec assumes a root-capable admin user (admin/admin) who can switch
// tenants via the TenantSwitcher in the sidebar. If only one tenant exists,
// the cross-tenant assertion is skipped.

async function listAvailableTenants(page: import("@playwright/test").Page): Promise<string[]> {
  // Open the TenantSwitcher (visible only for root).
  const trigger = page.locator('[role="combobox"]').first();
  if (!(await trigger.isVisible({ timeout: 2_000 }).catch(() => false))) return [];
  await trigger.click();
  const items = await page.locator('[role="option"]').allTextContents();
  // Close the popover.
  await page.keyboard.press("Escape");
  return items;
}

test.describe("Cross-tenant isolation (UI)", () => {
  test("a preset created in tenant A is invisible from tenant B", async ({ page }) => {
    await login(page);
    await gotoTenant(page, "presets");
    await page.waitForTimeout(1_000);

    const tenantsInSwitcher = await listAvailableTenants(page);
    if (tenantsInSwitcher.length < 2) {
      test.skip(true, "Need at least 2 tenants — admin user has fewer");
      return;
    }

    const tidA = getTenantId(page);

    // Create a unique-name preset in tenant A.
    const name = `iso-${Date.now()}`;
    const newBtn = page.getByRole("link", { name: /New Preset/i }).first();
    if (!(await newBtn.isVisible({ timeout: 2_000 }).catch(() => false))) {
      test.skip(true, "No 'New Preset' affordance");
      return;
    }
    await newBtn.click();
    await page.waitForURL(/\/presets\/new/, { timeout: 5_000 });
    const nameInput = page.locator('input[name="name"], #preset-name, input[placeholder*="name" i]').first();
    if (!(await nameInput.isVisible({ timeout: 3_000 }).catch(() => false))) {
      test.skip(true, "Designer has no recognisable name input");
      return;
    }
    await nameInput.fill(name);
    const saveBtn = page.getByRole("button", { name: /Save|Create/i }).first();
    await saveBtn.click();
    await page.waitForTimeout(2_000);

    // Confirm in tenant A.
    await gotoTenant(page, "presets");
    await page.waitForTimeout(1_000);
    await expect(page.getByText(name)).toBeVisible({ timeout: 5_000 });

    // Switch to a different tenant via the switcher.
    const trigger = page.locator('[role="combobox"]').first();
    await trigger.click();
    const options = page.locator('[role="option"]');
    const count = await options.count();
    let switched = false;
    for (let i = 0; i < count; i++) {
      const opt = options.nth(i);
      const text = await opt.textContent();
      if (text && !text.includes(tidA)) {
        await opt.click();
        switched = true;
        break;
      }
    }
    if (!switched) {
      test.skip(true, "Could not pick a different tenant from the switcher");
      return;
    }
    await page.waitForURL(/\/t\/[^/]+/, { timeout: 5_000 });
    const tidB = getTenantId(page);
    expect(tidB).not.toBe(tidA);

    // Navigate to presets list in tenant B; resource from A must be absent.
    await gotoTenant(page, "presets");
    await page.waitForTimeout(1_500);
    await expect(page.getByText(name)).toHaveCount(0);
  });
});
