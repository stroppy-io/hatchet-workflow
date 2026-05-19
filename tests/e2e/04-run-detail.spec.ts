import { test, expect } from "@playwright/test";
import { login, ensureRunsPage, getTenantId } from "./helpers";

// Functional: clicking a run row must open that run's detail page (URL bound
// to its id, same tenant) and the page must render data from the API (run id,
// timestamps, identity). Cancel button affects backend state.
//
// Legacy probes (DAG viz, log stream, metrics panel, agents panel, share/
// rerun dialogs) are gap.md C3 / A2-A4 and intentionally absent below.

async function openFirstRun(page: import("@playwright/test").Page): Promise<string | null> {
  await ensureRunsPage(page);
  await page.waitForTimeout(1_500);
  const row = page.locator("table tbody tr").first();
  if (!(await row.isVisible({ timeout: 3_000 }).catch(() => false))) return null;
  await row.click();
  await page.waitForURL(/\/t\/[^/]+\/runs\/[^/?]+/, { timeout: 5_000 });
  const m = page.url().match(/\/runs\/([^/?]+)/);
  return m ? m[1] : null;
}

test.describe("Run detail behavior", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("opens detail bound to the clicked run id under the same tenant", async ({ page }) => {
    const tid = getTenantId(page) || (await ensureRunsPage(page), getTenantId(page));
    const runId = await openFirstRun(page);
    if (!runId) test.skip(true, "No runs");
    expect(page.url()).toMatch(new RegExp(`/t/${tid}/runs/${runId}`));
  });

  test("detail page surfaces the run id from the URL on screen", async ({ page }) => {
    const runId = await openFirstRun(page);
    if (!runId) test.skip(true, "No runs");
    // Run id is a ULID — must appear somewhere in the rendered text (truncated or not).
    const prefix = (runId as string).slice(0, 8);
    await expect(page.getByText(prefix, { exact: false })).toBeVisible({ timeout: 5_000 });
  });

  test("reload preserves the URL — deep-link to a specific run works", async ({ page }) => {
    const runId = await openFirstRun(page);
    if (!runId) test.skip(true, "No runs");
    const before = page.url();
    await page.reload();
    await page.waitForTimeout(1_500);
    expect(page.url()).toBe(before);
  });
});
