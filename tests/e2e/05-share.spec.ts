import { test } from "@playwright/test";

// SharedRun page exists (page renders by token) but the share-link creation UI
// + REST endpoint /api/share/:token were removed in the refactor; getByToken is
// the only working surface. End-to-end UI flow is blocked until the share
// button + create-link RPC are re-wired. Tracked in docs/refactor-gap.md C5.
//
// Re-enable when:
//  - testing.CreateRunShare (or equivalent) exists on the backend
//  - RunDetail surfaces a Share button that hits it
test.describe.skip("Share flow (blocked: gap.md C5 — share-link create UI removed)", () => {
  test("creates a token and lets an anonymous viewer open it", () => {
    // Placeholder.
  });
});
