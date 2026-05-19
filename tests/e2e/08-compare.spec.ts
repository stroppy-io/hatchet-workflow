import { test, expect } from "@playwright/test";
import { login, gotoTenant } from "./helpers";

// Functional intent: compare two runs, see a non-empty metrics diff. Backend
// CompareRuns is wired but MetricsPort is nil (gap.md A4 / C4) — the diff is
// always empty. Only reachability + tenant scope are verifiable today; the
// real diff assertion is skipped pending A4.

test.describe("Compare reachability", () => {
  test("compare page loads in the active tenant scope", async ({ page }) => {
    await login(page);
    await gotoTenant(page, "compare");
    await page.waitForTimeout(1_000);
    expect(page.url()).toMatch(/\/t\/[^/]+\/compare/);
  });
});

test.describe.skip("Metrics diff (blocked: gap.md A4 — MetricsPort nil)", () => {
  test("comparing two completed runs returns a non-empty diff", () => {
    // Placeholder until backend wires the metrics port.
  });
});
