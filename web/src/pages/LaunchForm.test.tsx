import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { SchemaSchema } from "@stroppy-io/schemapb";

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
});
