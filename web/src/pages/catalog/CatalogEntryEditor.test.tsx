// The catalog edit page IS the embedded IDE now, not a CodeMirror form (see
// this file's own doc comment). Covers: the ticket -> iframe handshake
// (mint, success, 403, IDE-disabled 404), and the ORIGIN_LINKED fork-then-
// open guard that stands in for the IDE when a linked org row has no repo
// of its own yet to open.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";

const mockNavigate = vi.fn();
vi.mock("@/lib/router", () => ({
  useNavigate: () => mockNavigate,
  useTenantSlug: () => "acme",
  useParams: () => ({ kind: "providers", id: "entry-1" }),
}));

const {
  getInstanceEntry,
  getOrgEntry,
  getOrgEntryFiles,
  updateOrgEntry,
  checkInstanceBundle,
  checkCatalogBundle,
  createInstanceEntry,
  createOrgEntry,
} = vi.hoisted(() => ({
  getInstanceEntry: vi.fn(),
  getOrgEntry: vi.fn(),
  getOrgEntryFiles: vi.fn(),
  updateOrgEntry: vi.fn(),
  checkInstanceBundle: vi.fn(),
  checkCatalogBundle: vi.fn(),
  createInstanceEntry: vi.fn(),
  createOrgEntry: vi.fn(),
}));
vi.mock("@/services/catalog", () => ({
  getInstanceEntry,
  getOrgEntry,
  getOrgEntryFiles,
  updateOrgEntry,
  checkInstanceBundle,
  checkCatalogBundle,
  createInstanceEntry,
  createOrgEntry,
}));

const { mintIdeTicket } = vi.hoisted(() => ({ mintIdeTicket: vi.fn() }));
vi.mock("@/services/ide", async () => {
  const actual = await vi.importActual<typeof import("@/services/ide")>("@/services/ide");
  return { ...actual, mintIdeTicket };
});

import { CatalogEntryEditor } from "./CatalogEntryEditor";

function nativeEntry(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id: "entry-1",
    tenantId: "t1",
    name: "Yandex",
    description: "",
    authorId: "",
    level: "LEVEL_ORG" as const,
    kind: "KIND_PROVIDER" as const,
    slug: "yandex",
    version: 1,
    origin: "ORIGIN_NATIVE" as const,
    sourceEntryId: "",
    summary: { provides: ["machines"], providerSlug: "", machineGroupCount: 0, serviceCount: 0, compiles: true },
    ...overrides,
  };
}

beforeEach(() => {
  mockNavigate.mockClear();
  getInstanceEntry.mockReset();
  getOrgEntry.mockReset().mockResolvedValue(nativeEntry());
  getOrgEntryFiles.mockReset();
  updateOrgEntry.mockReset();
  mintIdeTicket.mockReset();
});

describe("CatalogEntryEditor — EDIT renders the embedded IDE, ticket -> iframe", () => {
  it("mints a ticket for the org scope target (singular kind) and points the iframe at the returned url", async () => {
    mintIdeTicket.mockResolvedValue({ ticket: "t1", url: "/ide/org/acme/provider/yandex?ticket=t1" });

    render(<CatalogEntryEditor scope="org" />);

    await waitFor(() => expect(mintIdeTicket).toHaveBeenCalled());
    expect(mintIdeTicket).toHaveBeenCalledWith(
      "/ide/org/acme/provider/yandex?folder=%2Fhome%2Fcoder%2Fproject%2Fproviders%2Fyandex",
    );

    const iframe = await screen.findByTestId("ide-iframe");
    expect(iframe).toHaveAttribute("src", "/ide/org/acme/provider/yandex?ticket=t1");
  });

  it("shows a real loading state before the ticket resolves, not a blank iframe", async () => {
    let resolveTicket: (v: { ticket: string; url: string }) => void = () => {};
    mintIdeTicket.mockReturnValue(new Promise((resolve) => (resolveTicket = resolve)));

    render(<CatalogEntryEditor scope="org" />);

    await waitFor(() => expect(screen.getByText(/Preparing your IDE session/i)).toBeInTheDocument());
    expect(screen.queryByTestId("ide-iframe")).not.toBeInTheDocument();

    resolveTicket({ ticket: "t1", url: "/ide/org/acme/provider/yandex?ticket=t1" });
    await screen.findByTestId("ide-iframe");
  });

  it("shows an explicit forbidden state on a 403, not a blank iframe", async () => {
    mintIdeTicket.mockRejectedValue(new Error("could not open IDE: 403 Forbidden"));

    render(<CatalogEntryEditor scope="org" />);

    await waitFor(() =>
      expect(screen.getByText(/not authorized to open this entry in the IDE/i)).toBeInTheDocument(),
    );
    expect(screen.queryByTestId("ide-iframe")).not.toBeInTheDocument();
  });

  it("shows an IDE-disabled state on a 404 (IDE_MANAGER_ENABLED off), not a blank iframe", async () => {
    mintIdeTicket.mockRejectedValue(new Error("could not open IDE: 404 Not Found"));

    render(<CatalogEntryEditor scope="org" />);

    await waitFor(() =>
      expect(screen.getByText(/embedded IDE is not enabled on this server/i)).toBeInTheDocument(),
    );
    expect(screen.queryByTestId("ide-iframe")).not.toBeInTheDocument();
  });
});

describe("CatalogEntryEditor — ORIGIN_LINKED fork-then-open guard", () => {
  it("does not mint a ticket or render an iframe for a linked entry — it has no repo yet", async () => {
    getOrgEntry.mockResolvedValue(nativeEntry({ origin: "ORIGIN_LINKED", sourceEntryId: "instance-yandex" }));

    render(<CatalogEntryEditor scope="org" />);

    await screen.findByRole("button", { name: /Fork & open in IDE/i });
    expect(mintIdeTicket).not.toHaveBeenCalled();
    expect(screen.queryByTestId("ide-iframe")).not.toBeInTheDocument();
  });

  it("forks (via updateOrgEntry with the entry's own files) then navigates to the forked entry's edit route", async () => {
    getOrgEntry.mockResolvedValue(nativeEntry({ origin: "ORIGIN_LINKED", sourceEntryId: "instance-yandex" }));
    getOrgEntryFiles.mockResolvedValue({ "manifest.yaml": "name: yandex\n" });
    updateOrgEntry.mockResolvedValue(nativeEntry({ id: "forked-1", origin: "ORIGIN_FORKED" }));

    render(<CatalogEntryEditor scope="org" />);

    const forkButton = await screen.findByRole("button", { name: /Fork & open in IDE/i });
    fireEvent.click(forkButton);

    await waitFor(() =>
      expect(updateOrgEntry).toHaveBeenCalledWith("acme", "KIND_PROVIDER", "entry-1", {
        "manifest.yaml": "name: yandex\n",
      }),
    );
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith("/catalog/providers/forked-1/edit", { replace: true }),
    );
  });

  it("surfaces a fork failure instead of silently doing nothing", async () => {
    getOrgEntry.mockResolvedValue(nativeEntry({ origin: "ORIGIN_LINKED" }));
    getOrgEntryFiles.mockRejectedValue(new Error("network down"));

    render(<CatalogEntryEditor scope="org" />);

    const forkButton = await screen.findByRole("button", { name: /Fork & open in IDE/i });
    fireEvent.click(forkButton);

    await waitFor(() => expect(screen.getByText("network down")).toBeInTheDocument());
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
