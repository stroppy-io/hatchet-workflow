import { test } from "@playwright/test";

// Quota enforcement depended on the legacy wizard's Step 2 filtering DB kinds
// by the tenant's allowed_db_kinds setting. The current NewRun page does not
// expose a DB-kind selector at all (gap.md C3 — wizard lost). Re-enable when
// either:
//  (a) the wizard returns, or
//  (b) backend rejects CreateTestRun with a quota error and the form surfaces it.
test.describe.skip("Quota enforcement on NewRun (blocked: gap.md C3 — wizard absent)", () => {
  test("allowed_db_kinds filters the DB selector", () => {
    // Placeholder.
  });
});
