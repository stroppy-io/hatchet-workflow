import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema, FieldErrorSchema } from "@stroppy-io/schemapb";

// Mock only the RPC transport (recipeClient) + the tenant slug->id resolver —
// fetchLaunchFormSchema/startRun (services/recipe.ts) and LaunchFormRenderer
// (real schemapb WASM engine) run for real, same as LaunchFormRenderer.test.tsx.
const mockNavigate = vi.fn();
vi.mock("@/lib/router", () => ({
  useNavigate: () => mockNavigate,
  useParams: () => ({ id: "r1" }),
  useTenantSlug: () => "acme",
}));

vi.mock("@/services/client", () => ({
  recipeClient: {
    launchFormSchema: vi.fn(),
    startRun: vi.fn(),
  },
  dslClient: {},
}));
vi.mock("@/services/tenant", () => ({ resolveTenantId: vi.fn().mockResolvedValue("t1") }));

import { recipeClient } from "@/services/client";
import { LaunchForm } from "./LaunchForm";

function schemaWithOneField() {
  return create(SchemaSchema, {
    id: { namespace: "stroppy.test", name: "launch", version: "v1" },
    fields: [{ name: "db_version", kind: { case: "string", value: { default: "16" } } }],
  });
}

describe("LaunchForm", () => {
  beforeEach(() => {
    mockNavigate.mockClear();
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
});
