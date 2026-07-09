// RecipeRuns table parity: asserts the DSL-era Run.Summary facets (db kind,
// provider, node count, progress, recipe link) that ref's Runs.tsx table
// carried are actually rendered — the product complaint this rebuild fixes
// was "the runs table has no content" (see
// .superpowers/sdd/runs-parity-report.md). Also covers the status filter
// chips and the cancel/delete row actions, gated by actionsForStatus.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { ConfirmProvider } from "@/components/ui/confirm-dialog";
import type { RunVM } from "@/services/runs";

const mockNavigate = vi.fn();
vi.mock("@/lib/router", () => ({
  Link: ({ to, children, onClick }: { to: string; children: React.ReactNode; onClick?: (e: React.MouseEvent) => void }) => (
    <a href={to} onClick={onClick}>
      {children}
    </a>
  ),
  useNavigate: () => mockNavigate,
  useTenantSlug: () => "acme",
}));

const { listRuns, cancelRun, deleteRun, listRecipes } = vi.hoisted(() => ({
  listRuns: vi.fn(),
  cancelRun: vi.fn(),
  deleteRun: vi.fn(),
  listRecipes: vi.fn(),
}));
vi.mock("@/services/recipe", () => ({ listRuns, cancelRun, deleteRun, listRecipes }));

import { RecipeRuns } from "./RecipeRuns";

function runningRun(overrides: Partial<RunVM> = {}): RunVM {
  return {
    id: "run-aaaaaaaa-1111",
    name: "postgres-ha smoke",
    authorId: "",
    createdAt: "2026-07-01T00:00:00Z",
    status: "running",
    dbKind: "postgres",
    dbVersion: "16",
    workload: "insert+select",
    stroppyVersion: "1.2.3",
    protocol: "pg",
    trigger: "manual",
    topologyLabel: "docker · 3 nodes",
    nodeCount: 3,
    progressPct: 42,
    startedAt: "2026-07-01T00:05:00Z",
    finishedAt: undefined,
    durationSec: 90,
    workflowId: "recipe-1",
    provider: "docker",
    dbPresetId: "",
    workloadPresetId: "",
    testPresetId: "",
    favorite: false,
    deleted: false,
    ...overrides,
  };
}

beforeEach(() => {
  mockNavigate.mockClear();
  listRuns.mockReset().mockResolvedValue([runningRun()]);
  cancelRun.mockReset().mockResolvedValue(undefined);
  deleteRun.mockReset().mockResolvedValue(undefined);
  listRecipes.mockReset().mockResolvedValue([{ id: "recipe-1", name: "postgres-ha", version: 1, provider: "docker", machineGroupCount: 1, serviceCount: 2, compiles: true, files: {} }]);
});

function renderPage() {
  return render(
    <ConfirmProvider>
      <RecipeRuns />
    </ConfirmProvider>,
  );
}

describe("RecipeRuns table", () => {
  it("renders the db kind, node count, provider, progress and recipe name columns", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("postgres-ha smoke")).toBeInTheDocument());

    expect(screen.getByText("PostgreSQL")).toBeInTheDocument();
    expect(screen.getByText("16")).toBeInTheDocument();
    expect(screen.getByText(/3 nodes/)).toBeInTheDocument();
    expect(screen.getByText("docker")).toBeInTheDocument();
    expect(screen.getByText("42%")).toBeInTheDocument();
    // Recipe column resolves workflowId -> recipe name via listRecipes.
    expect(await screen.findByText("postgres-ha")).toBeInTheDocument();
  });

  it("filters rows by status via the filter chips", async () => {
    listRuns.mockResolvedValue([
      runningRun({ id: "run-1", name: "run one", status: "running" }),
      runningRun({ id: "run-2", name: "run two", status: "completed", progressPct: 100 }),
    ]);
    renderPage();
    await waitFor(() => expect(screen.getByText("run one")).toBeInTheDocument());
    expect(screen.getByText("run two")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "completed" }));

    expect(screen.queryByText("run one")).not.toBeInTheDocument();
    expect(screen.getByText("run two")).toBeInTheDocument();
  });

  it("cancelling a running run confirms then calls RecipeService.CancelRun", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText("postgres-ha smoke")).toBeInTheDocument());

    fireEvent.click(screen.getByTitle("Cancel run"));
    await waitFor(() => expect(screen.getByText("Cancel run?")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Cancel run" }));

    await waitFor(() => expect(cancelRun).toHaveBeenCalledWith("acme", "run-aaaaaaaa-1111"));
  });

  it("a terminal run offers Rerun and Delete, not Cancel", async () => {
    listRuns.mockResolvedValue([runningRun({ status: "completed", progressPct: 100, finishedAt: "2026-07-01T00:10:00Z" })]);
    renderPage();
    await waitFor(() => expect(screen.getByText("postgres-ha smoke")).toBeInTheDocument());

    expect(screen.queryByTitle("Cancel run")).not.toBeInTheDocument();
    expect(screen.getByTitle("Rerun this recipe")).toBeInTheDocument();
    expect(screen.getByTitle("Delete run")).toBeInTheDocument();

    fireEvent.click(screen.getByTitle("Rerun this recipe"));
    expect(mockNavigate).toHaveBeenCalledWith("/recipes/recipe-1/launch?from=run-aaaaaaaa-1111");
  });

  it("deleting a run confirms then calls RecipeService.DeleteRun", async () => {
    listRuns.mockResolvedValue([runningRun({ status: "failed", progressPct: 20 })]);
    renderPage();
    await waitFor(() => expect(screen.getByText("postgres-ha smoke")).toBeInTheDocument());

    fireEvent.click(screen.getByTitle("Delete run"));
    await waitFor(() => expect(screen.getByText("Delete run?")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));

    await waitFor(() => expect(deleteRun).toHaveBeenCalledWith("acme", "run-aaaaaaaa-1111"));
  });
});
