// instanceIdeUrl/orgIdeUrl scope-string construction and the ticket-mint
// handshake mintIdeTicket/openInIde build on. The scope segment MUST stay
// singular ("provider"/"workflow") even though every caller works off the
// plural list-tab name ("providers"/"workflows") — a prior version of these
// builders passed the plural straight through and 404'd at the gateway
// (internal/ide.ParseScope only recognizes the singular form). The `folder`
// deep-link query param is the one place the plural form legitimately
// belongs (it mirrors the repo's own on-disk providers/<slug> |
// workflows/<slug> layout).

import { describe, it, expect, vi, beforeEach } from "vitest";

const { getAccessToken } = vi.hoisted(() => ({ getAccessToken: vi.fn() }));
vi.mock("@/services/client", () => ({ getAccessToken }));

import { instanceIdeUrl, mintIdeTicket, openInIde, orgIdeUrl, recipeIdeUrl } from "./ide";

describe("instanceIdeUrl / orgIdeUrl — scope-string construction", () => {
  it("uses the singular entry-kind segment for a provider, never the plural tab name", () => {
    expect(instanceIdeUrl("providers", "yandex")).toBe(
      "/ide/instance/provider/yandex?folder=%2Fhome%2Fcoder%2Fproject",
    );
  });

  it("uses the singular entry-kind segment for a workflow, never the plural tab name", () => {
    expect(instanceIdeUrl("workflows", "smoke-test")).toBe(
      "/ide/instance/workflow/smoke-test?folder=%2Fhome%2Fcoder%2Fproject",
    );
  });

  it("orgIdeUrl inserts the org slug before the singular entry-kind segment", () => {
    expect(orgIdeUrl("acme", "providers", "yandex")).toBe(
      "/ide/org/acme/provider/yandex?folder=%2Fhome%2Fcoder%2Fproject",
    );
  });

  it("orgIdeUrl for a workflow also stays singular in the scope segment", () => {
    expect(orgIdeUrl("acme", "workflows", "smoke-test")).toBe(
      "/ide/org/acme/workflow/smoke-test?folder=%2Fhome%2Fcoder%2Fproject",
    );
  });

  // recipeIdeUrl is org-only (a recipe has no LEVEL_INSTANCE equivalent —
  // see internal/ide.EntryKindRecipe's doc): there is no
  // instanceRecipeIdeUrl for the same reason instanceIdeUrl's tab param
  // excludes "recipes" at compile time.
  it("recipeIdeUrl builds an org-scoped /ide/org/<slug>/recipe/<name> URL", () => {
    expect(recipeIdeUrl("acme", "pg-ha")).toBe(
      "/ide/org/acme/recipe/pg-ha?folder=%2Fhome%2Fcoder%2Fproject",
    );
  });
});

describe("mintIdeTicket / openInIde — the browser-auth handshake", () => {
  beforeEach(() => {
    getAccessToken.mockReset();
    vi.stubGlobal("fetch", vi.fn());
    vi.stubGlobal("open", vi.fn());
  });

  it("throws without ever calling fetch when there is no access token", async () => {
    getAccessToken.mockReturnValue(null);
    await expect(mintIdeTicket("/ide/instance/provider/yandex")).rejects.toThrow("not signed in");
    expect(fetch).not.toHaveBeenCalled();
  });

  it("calls /api/ide/ticket with the Bearer token and the encoded target, and returns the ticket URL", async () => {
    getAccessToken.mockReturnValue("tok-123");
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({ ticket: "t1", url: "/ide/instance/provider/yandex?ticket=t1" }),
    });

    const result = await mintIdeTicket("/ide/instance/provider/yandex?folder=x");

    expect(fetch).toHaveBeenCalledWith(
      "/api/ide/ticket?target=%2Fide%2Finstance%2Fprovider%2Fyandex%3Ffolder%3Dx",
      { headers: { Authorization: "Bearer tok-123" } },
    );
    expect(result).toEqual({ ticket: "t1", url: "/ide/instance/provider/yandex?ticket=t1" });
  });

  it("surfaces the status code on a non-ok response (e.g. a 403 from an unauthorized scope)", async () => {
    getAccessToken.mockReturnValue("tok-123");
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: false, status: 403, statusText: "Forbidden" });

    await expect(mintIdeTicket("/ide/org/acme/provider/yandex")).rejects.toThrow("403");
  });

  it("surfaces a 404 when the IDE manager is disabled server-side", async () => {
    getAccessToken.mockReturnValue("tok-123");
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({ ok: false, status: 404, statusText: "Not Found" });

    await expect(mintIdeTicket("/ide/instance/provider/yandex")).rejects.toThrow("404");
  });

  it("openInIde navigates a new tab to the minted ticket URL, never to the raw target", async () => {
    getAccessToken.mockReturnValue("tok-123");
    (fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
      ok: true,
      json: async () => ({ ticket: "t1", url: "/ide/instance/provider/yandex?ticket=t1" }),
    });

    await openInIde("/ide/instance/provider/yandex");

    expect(open).toHaveBeenCalledWith("/ide/instance/provider/yandex?ticket=t1", "_blank", "noreferrer");
  });
});
