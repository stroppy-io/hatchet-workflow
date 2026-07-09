// OrgCatalog fork-on-edit confirmation flow: deleting a LINKED entry must
// warn that only the org's own live link is removed (the instance source is
// untouched), distinctly from deleting a NATIVE/FORKED row. This is the
// user-facing half of the fork-on-edit contract documented in
// services/catalog.ts (server forks implicitly on UpdateOrgProvider/
// UpdateOrgWorkflow; there is no separate fork RPC) — the Edit button's title
// on a LINKED row is the other half, asserted here too.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { ConfirmProvider } from "@/components/ui/confirm-dialog";

const mockNavigate = vi.fn();
let searchParams = new URLSearchParams();
vi.mock("@/lib/router", () => ({
  Link: ({ to, children }: { to: string; children: React.ReactNode }) => <a href={to}>{children}</a>,
  useNavigate: () => mockNavigate,
  useTenantSlug: () => "acme",
  useSearchParams: () => [searchParams, vi.fn()],
}));

const { listOrgEntries, deleteOrgEntry, listInstanceEntries, linkInstanceEntry } = vi.hoisted(() => ({
  listOrgEntries: vi.fn(),
  deleteOrgEntry: vi.fn(),
  listInstanceEntries: vi.fn(),
  linkInstanceEntry: vi.fn(),
}));
vi.mock("@/services/catalog", () => ({
  listOrgEntries,
  deleteOrgEntry,
  listInstanceEntries,
  linkInstanceEntry,
}));

const { getOrg } = vi.hoisted(() => ({ getOrg: vi.fn() }));
vi.mock("@/services/org", async () => {
  const actual = await vi.importActual<typeof import("@/services/org")>("@/services/org");
  return {
    ...actual,
    getOrgProvider: () => ({ getOrg }),
  };
});

import { OrgCatalog } from "./OrgCatalog";

const OWNER_ROLE = {
  id: "role-owner",
  name: "Owner",
  scope: "SCOPE_TENANT" as const,
  tenantId: "t1",
  isSystem: true,
  permissions: [
    { resource: "RESOURCE_PROVIDER", action: "ACTION_MANAGE" },
    { resource: "RESOURCE_WORKFLOW", action: "ACTION_MANAGE" },
  ],
};

function linkedEntry() {
  return {
    id: "linked-1",
    tenantId: "t1",
    name: "Yandex",
    description: "",
    authorId: "",
    level: "LEVEL_ORG" as const,
    kind: "KIND_PROVIDER" as const,
    slug: "yandex",
    version: 1,
    origin: "ORIGIN_LINKED" as const,
    sourceEntryId: "instance-yandex",
    summary: { provides: ["machines"], providerSlug: "", machineGroupCount: 0, serviceCount: 0, compiles: true },
  };
}

beforeEach(() => {
  mockNavigate.mockClear();
  searchParams = new URLSearchParams();
  listOrgEntries.mockReset().mockResolvedValue([linkedEntry()]);
  deleteOrgEntry.mockReset().mockResolvedValue(undefined);
  listInstanceEntries.mockReset().mockResolvedValue([]);
  linkInstanceEntry.mockReset();
  getOrg.mockReset().mockResolvedValue({
    tenant: { id: "t1", name: "Acme", ownerAccountId: "", slug: "acme" },
    currentRoles: [OWNER_ROLE],
    memberships: [],
    roles: [OWNER_ROLE],
    settings: { entity: {} },
  });
});

function renderPage() {
  return render(
    <ConfirmProvider>
      <OrgCatalog />
    </ConfirmProvider>,
  );
}

describe("OrgCatalog — linked-entry lineage and fork-on-edit warnings", () => {
  it("shows a Linked badge for a linked entry", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Yandex")).toBeInTheDocument());
    const row = screen.getByText("Yandex").closest("tr");
    expect(row).not.toBeNull();
    expect(within(row as HTMLElement).getByText("Linked")).toBeInTheDocument();
  });

  it("the Edit button on a linked row is titled to warn it will fork on save", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Yandex")).toBeInTheDocument());
    const editButton = screen.getByLabelText("Edit");
    expect(editButton).toHaveAttribute(
      "title",
      "Editing a linked entry forks it into an independent copy",
    );
  });

  it("deleting a linked entry asks for confirmation and warns only the org's link is removed", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("Yandex")).toBeInTheDocument());

    fireEvent.click(screen.getByLabelText("Delete"));

    await waitFor(() => expect(screen.getByText("Delete Yandex?")).toBeInTheDocument());
    expect(
      screen.getByText(/LIVE LINK.*"yandex".*instance catalog entry itself is unaffected/i),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(deleteOrgEntry).toHaveBeenCalledWith("acme", "KIND_PROVIDER", "linked-1"));
  });
});
