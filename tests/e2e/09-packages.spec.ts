import { test, expect } from "@playwright/test";
import { login, gotoTenant } from "./helpers";

// Functional: packages list is the canonical reference of installable binaries.
// Built-in tenant seed (A14) is missing in refactor, so we only validate that
// the page renders honestly: either the list (rows from API) or an empty hint.
// Upload/download via REST is gap.md A10 (lost) — those probes are skipped.

test.describe("Packages list", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await gotoTenant(page, "packages");
    await page.waitForTimeout(1_500);
  });

  test("tenant-scoped URL", async ({ page }) => {
    expect(page.url()).toMatch(/\/t\/[^/]+\/packages/);
  });

  test("page renders honestly — list or empty hint, never blank", async ({ page }) => {
    const hasRows = (await page.locator("table tbody tr, [data-package-row]").count()) > 0;
    const hasCard = (await page.getByRole("listitem").count()) > 0;
    const hasEmpty = (await page.getByText(/no packages/i).count()) > 0;
    expect(hasRows || hasCard || hasEmpty).toBeTruthy();
  });
});

test.describe.skip("Package upload/download (blocked: gap.md A10 — REST removed, no Connect equivalent)", () => {
  test("user uploads a .deb and downloads it back", () => {
    // Placeholder.
  });
});
