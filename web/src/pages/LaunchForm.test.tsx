import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { create, toJson } from "@bufbuild/protobuf";
import { SchemaSchema, FieldErrorSchema, FilledSchema } from "@stroppy-io/schemapb";

// Mock only the RPC transport (recipeClient) + the tenant slug->id resolver —
// fetchLaunchFormSchema/startRun (services/recipe.ts) and LaunchFormRenderer
// (real schemapb WASM engine) run for real, same as LaunchFormRenderer.test.tsx.
//
// searchParams is a module-level mutable handle so individual tests can set
// `?from=<runId>` (the Rerun flow's own signal, see RunDetail.tsx's onRerun)
// before rendering, mirroring how react-router's real useSearchParams reads
// off the current URL.
let searchParams = new URLSearchParams();
const mockNavigate = vi.fn();
vi.mock("@/lib/router", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ id: "r1" }),
  useTenantSlug: () => "acme",
  useSearchParams: () => [searchParams, vi.fn()],
}));

vi.mock("@/services/client", () => ({
  recipeClient: {
    launchFormSchema: vi.fn(),
    startRun: vi.fn(),
  },
  dslClient: {},
}));
vi.mock("@/services/tenant", () => ({ resolveTenantId: vi.fn().mockResolvedValue("t1") }));

const mockGetRunOverview = vi.fn();
vi.mock("@/services/run_overview", () => ({
  getRunOverview: (...args: unknown[]) => mockGetRunOverview(...args),
}));

import { recipeClient } from "@/services/client";
import { LaunchForm } from "./LaunchForm";

function schemaWithOneField() {
  return create(SchemaSchema, {
    id: { namespace: "stroppy.test", name: "launch", version: "v1" },
    fields: [{ name: "db_version", kind: { case: "string", value: { default: "16" } } }],
  });
}

function schemaWithProvider() {
  return create(SchemaSchema, {
    id: { namespace: "stroppy.test", name: "launch", version: "v1" },
    fields: [
      { name: "db_version", kind: { case: "string", value: { default: "16" } } },
      {
        name: "provider",
        kind: {
          case: "object",
          value: {
            schema: create(SchemaSchema, {
              id: { namespace: "stroppy.test", name: "provider", version: "v1" },
              fields: [{ name: "zone", kind: { case: "string", value: {} } }],
            }),
          },
        },
      },
    ],
  });
}

describe("LaunchForm", () => {
  beforeEach(() => {
    mockNavigate.mockClear();
    mockGetRunOverview.mockReset();
    searchParams = new URLSearchParams();
    vi.mocked(recipeClient.startRun).mockReset();
    vi.mocked(recipeClient.launchFormSchema).mockReset();
  });

  it("fetches the recipe's launch-form schema and renders its fields", async () => {
    vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
      schema: schemaWithOneField(),
    } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);

    render(<LaunchForm />);

    expect(screen.getByText(/loading launch form/i)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
    expect(recipeClient.launchFormSchema).toHaveBeenCalledWith({ tenantId: "t1", recipeId: "r1" });
  });

  it("surfaces an error state when the schema fetch fails", async () => {
    vi.mocked(recipeClient.launchFormSchema).mockRejectedValue(new Error("recipe not found"));

    render(<LaunchForm />);

    await waitFor(() => expect(screen.getByText("recipe not found")).toBeInTheDocument());
  });

  it("submits the baked payload to startRun and navigates to the new run on success", async () => {
    vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
      schema: schemaWithOneField(),
    } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);
    vi.mocked(recipeClient.startRun).mockResolvedValue({
      run: { entity: { id: "run-1" } },
      fieldErrors: [],
    } as unknown as Awaited<ReturnType<typeof recipeClient.startRun>>);

    render(<LaunchForm />);

    await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
    // Give the WASM engine a beat to finish loading (not just the field
    // rendering, which happens before the engine is ready — see
    // LaunchFormRenderer.test.tsx's own "before the WASM engine finishes
    // loading" note): handleSubmit needs the engine to be ready to bake
    // anything at all, and there is no externally-observable "ready" signal
    // on the DOM to wait on directly.
    await new Promise((r) => setTimeout(r, 300));
    fireEvent.click(screen.getByRole("button", { name: /launch/i }));

    await waitFor(() => expect(recipeClient.startRun).toHaveBeenCalled(), { timeout: 3000 });
    const call = vi.mocked(recipeClient.startRun).mock.calls[0][0] as { tenantId: string; recipeId: string };
    expect(call.tenantId).toBe("t1");
    expect(call.recipeId).toBe("r1");
    await waitFor(() => expect(mockNavigate).toHaveBeenCalledWith("/runs/run-1"));
  });

  it("surfaces per-field errors from a server-side bake failure without navigating", async () => {
    vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
      schema: schemaWithOneField(),
    } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);
    vi.mocked(recipeClient.startRun).mockResolvedValue({
      run: undefined,
      fieldErrors: [create(FieldErrorSchema, { field: "db_version", message: "must not be empty" })],
    } as unknown as Awaited<ReturnType<typeof recipeClient.startRun>>);

    render(<LaunchForm />);

    await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
    await new Promise((r) => setTimeout(r, 300));
    fireEvent.click(screen.getByRole("button", { name: /launch/i }));

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("must not be empty"), { timeout: 3000 });
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  // Finding I2: RunDetail's Rerun action navigates here with ?from=<runId>
  // instead of calling StartRun directly with no Filled (which silently bakes
  // nil server-side and launches the bundle's static defaults). These cover
  // the decision: rerun prefills the CURRENT recipe's launch form from the
  // originating run's stored Baked snapshot, never bypasses re-Baking.
  describe("rerun prefill (?from=<runId>)", () => {
    it("prefills the form from the originating run's stored Baked values, split across the top-level/provider fields", async () => {
      searchParams = new URLSearchParams("from=run-0");
      vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
        schema: schemaWithProvider(),
      } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);
      mockGetRunOverview.mockResolvedValue({
        run: { baked: { values: { db_version: "17", provider: { zone: "ru-central1" } } } },
      });

      render(<LaunchForm />);

      await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
      expect(mockGetRunOverview).toHaveBeenCalledWith("acme", "run-0");
      expect((screen.getByLabelText("db_version") as HTMLInputElement).value).toBe("17");
      expect((screen.getByLabelText("provider.zone") as HTMLInputElement).value).toBe("ru-central1");
    });

    it("sends the EDITED value, not the prefilled one, when a prefilled field is changed before Launch", async () => {
      searchParams = new URLSearchParams("from=run-0");
      vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
        schema: schemaWithOneField(),
      } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);
      mockGetRunOverview.mockResolvedValue({
        run: { baked: { values: { db_version: "17" } } },
      });
      vi.mocked(recipeClient.startRun).mockResolvedValue({
        run: { entity: { id: "run-2" } },
        fieldErrors: [],
      } as unknown as Awaited<ReturnType<typeof recipeClient.startRun>>);

      render(<LaunchForm />);

      await waitFor(() => expect((screen.getByLabelText("db_version") as HTMLInputElement).value).toBe("17"));
      fireEvent.change(screen.getByLabelText("db_version"), { target: { value: "18" } });
      await new Promise((r) => setTimeout(r, 300));
      fireEvent.click(screen.getByRole("button", { name: /launch/i }));

      await waitFor(() => expect(recipeClient.startRun).toHaveBeenCalled(), { timeout: 3000 });
      const call = vi.mocked(recipeClient.startRun).mock.calls[0][0] as { filled?: unknown };
      const filledJson = toJson(FilledSchema, call.filled as Parameters<typeof toJson<typeof FilledSchema>>[1]) as {
        values?: { db_version?: string };
      };
      expect(filledJson.values?.db_version).toBe("18");
    });

    it("still reruns cleanly (empty/default form) when the originating run has no stored Baked (Baked == nil)", async () => {
      searchParams = new URLSearchParams("from=run-0");
      vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
        schema: schemaWithOneField(),
      } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);
      mockGetRunOverview.mockResolvedValue({ run: { baked: undefined } });

      render(<LaunchForm />);

      await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
      // Falls back to today's no-prefill behavior (identical to the plain
      // /recipes/:id/launch path with no `from` at all, exercised by "fetches
      // the recipe's launch-form schema and renders its fields" above) — no
      // crash, no stale/garbage value from a run with nothing to prefill.
      expect((screen.getByLabelText("db_version") as HTMLInputElement).value).toBe("");
    });

    it("still reruns cleanly when the originating run lookup itself fails (deleted/cross-tenant run id)", async () => {
      searchParams = new URLSearchParams("from=someone-elses-run");
      vi.mocked(recipeClient.launchFormSchema).mockResolvedValue({
        schema: schemaWithOneField(),
      } as Awaited<ReturnType<typeof recipeClient.launchFormSchema>>);
      mockGetRunOverview.mockRejectedValue(new Error("not found"));

      render(<LaunchForm />);

      await waitFor(() => expect(screen.getByLabelText("db_version")).toBeInTheDocument());
      expect((screen.getByLabelText("db_version") as HTMLInputElement).value).toBe("");
    });
  });
});
